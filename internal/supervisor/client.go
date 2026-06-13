package supervisor

import (
        "bytes"
        "encoding/xml"
        "fmt"
        "io"
        "net/http"
        "strings"
        "time"

        "shark-dashboard/internal/model"
)

// Client connects to Supervisord via XML-RPC over TCP.
type Client struct {
        url    string
        client *http.Client
}

// NewClient creates a new Supervisord XML-RPC client.
func NewClient(addr string) *Client {
        return &Client{
                url: addr + "/RPC2",
                client: &http.Client{
                        Timeout: 2 * time.Second,
                },
        }
}

// GetAllProcessInfo fetches info about all managed processes.
// Returns empty slice (never nil) on error to prevent JSON null.
func (c *Client) GetAllProcessInfo() ([]model.ProcessInfo, error) {
        // Build XML-RPC request
        reqBody := `<?xml version="1.0"?><methodCall><methodName>supervisor.getAllProcessInfo</methodName><params></params></methodCall>`

        resp, err := c.client.Post(c.url, "text/xml", strings.NewReader(reqBody))
        if err != nil {
                return make([]model.ProcessInfo, 0), fmt.Errorf("supervisord connection failed: %w", err)
        }
        defer resp.Body.Close()

        if resp.StatusCode != 200 {
                return make([]model.ProcessInfo, 0), fmt.Errorf("supervisord returned status %d", resp.StatusCode)
        }

        body, err := io.ReadAll(resp.Body)
        if err != nil {
                return make([]model.ProcessInfo, 0), fmt.Errorf("failed to read response: %w", err)
        }

        return parseProcessInfo(body)
}

// XML-RPC response parsing structures
type xmlMethodResponse struct {
        XMLName xml.Name `xml:"methodResponse"`
        Params  xmlParams `xml:"params"`
}

type xmlParams struct {
        Param xmlParam `xml:"param"`
}

type xmlParam struct {
        Value xmlValue `xml:"value"`
}

type xmlValue struct {
        Array  *xmlArray  `xml:"array"`
        String string     `xml:"string"`
}

type xmlArray struct {
        Data xmlData `xml:"data"`
}

type xmlData struct {
        Values []xmlValue `xml:"value"`
}

// processStruct represents a single process info struct in XML-RPC response
type processStruct struct {
        Members []xmlMember `xml:"member"`
}

type xmlMember struct {
        Name  string    `xml:"name"`
        Value xmlValue2 `xml:"value"`
}

type xmlValue2 struct {
        Int    int    `xml:"int"`
        String string `xml:"string"`
}

func parseProcessInfo(data []byte) ([]model.ProcessInfo, error) {
        // Parse the XML-RPC response manually since Go's encoding/xml
        // doesn't handle the deeply nested XML-RPC struct format well.
        processes := make([]model.ProcessInfo, 0) // never nil → JSON []

        // Simple approach: extract process blocks using string parsing
        // Each process is a <struct>...</struct> block
        content := string(data)

        // Find all <struct> blocks
        structs := extractStructs(content)

        for _, s := range structs {
                p := parseStructToProcess(s)
                processes = append(processes, p)
        }

        if len(processes) == 0 {
                // Try parsing as XML for structured access
                var resp xmlMethodResponse
                if err := xml.Unmarshal(data, &resp); err == nil {
                        if resp.Params.Param.Value.Array != nil {
                                for _, v := range resp.Params.Param.Value.Array.Data.Values {
                                        // Each value contains a struct with members
                                        structXML, err := xml.Marshal(v)
                                        if err != nil {
                                                continue
                                        }
                                        p := parseStructToProcess(string(structXML))
                                        processes = append(processes, p)
                                }
                        }
                }
        }

        return processes, nil
}

func extractStructs(content string) []string {
        var structs []string
        startTag := "<struct>"
        endTag := "</struct>"

        idx := 0
        for {
                start := strings.Index(content[idx:], startTag)
                if start == -1 {
                        break
                }
                start += idx
                end := strings.Index(content[start:], endTag)
                if end == -1 {
                        break
                }
                end += start + len(endTag)
                structs = append(structs, content[start:end])
                idx = end
        }

        return structs
}

func parseStructToProcess(s string) model.ProcessInfo {
        p := model.ProcessInfo{}

        members := extractMembers(s)
        for _, m := range members {
                name := extractTagContent(m, "<name>", "</name>")
                value := extractValue(m)

                switch name {
                case "name":
                        p.Name = value
                case "group":
                        p.Group = value
                case "pid":
                        p.PID = atoiSafe(value)
                case "state":
                        p.State = atoiSafe(value)
                case "statename":
                        p.StateName = value
                case "start":
                        p.StartTime = atoi64Safe(value)
                case "exitstatus":
                        p.ExitStatus = atoiSafe(value)
                case "description":
                        p.Description = value
                }
        }

        // Fallback: if statename is empty, derive from state code
        if p.StateName == "" {
                if name, ok := model.StateNameMap[p.State]; ok {
                        p.StateName = name
                } else {
                        p.StateName = fmt.Sprintf("UNKNOWN(%d)", p.State)
                }
        }

        return p
}

func extractMembers(s string) []string {
        var members []string
        startTag := "<member>"
        endTag := "</member>"

        idx := 0
        for {
                start := strings.Index(s[idx:], startTag)
                if start == -1 {
                        break
                }
                start += idx
                end := strings.Index(s[start:], endTag)
                if end == -1 {
                        break
                }
                end += start + len(endTag)
                members = append(members, s[start:end])
                idx = end
        }

        return members
}

func extractTagContent(s, startTag, endTag string) string {
        start := strings.Index(s, startTag)
        if start == -1 {
                return ""
        }
        start += len(startTag)
        end := strings.Index(s[start:], endTag)
        if end == -1 {
                return ""
        }
        return s[start : start+end]
}

func extractValue(member string) string {
        // Try <int> first
        if v := extractTagContent(member, "<int>", "</int>"); v != "" {
                return v
        }
        // Try <i4>
        if v := extractTagContent(member, "<i4>", "</i4>"); v != "" {
                return v
        }
        // Try <string>
        if v := extractTagContent(member, "<string>", "</string>"); v != "" {
                return v
        }
        // Try <double>
        if v := extractTagContent(member, "<double>", "</double>"); v != "" {
                return v
        }
        // Try <boolean>
        if v := extractTagContent(member, "<boolean>", "</boolean>"); v != "" {
                return v
        }
        return ""
}

func atoiSafe(s string) int {
        s = strings.TrimSpace(s)
        var v int
        fmt.Sscanf(s, "%d", &v)
        return v
}

func atoi64Safe(s string) int64 {
        s = strings.TrimSpace(s)
        var v int64
        fmt.Sscanf(s, "%d", &v)
        return v
}

// VerifyConnection checks if Supervisord is reachable.
func (c *Client) VerifyConnection() error {
        reqBody := `<?xml version="1.0"?><methodCall><methodName>supervisor.getSupervisorVersion</methodName><params></params></methodCall>`

        resp, err := c.client.Post(c.url, "text/xml", bytes.NewReader([]byte(reqBody)))
        if err != nil {
                return fmt.Errorf("cannot connect to supervisord at %s: %w", c.url, err)
        }
        defer resp.Body.Close()

        if resp.StatusCode != 200 {
                return fmt.Errorf("supervisord returned status %d", resp.StatusCode)
        }

        return nil
}
