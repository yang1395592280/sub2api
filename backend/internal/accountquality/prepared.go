package accountquality

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
)

type resolution struct {
	target Target
	err    error
}
type preparedKey struct{}

func resolutionKey(id, source int64, c Condition) string { return Hash([]any{id, source, c}) }
func (s *Service) resolveTarget(ctx context.Context, id, source int64, c Condition) (Target, error) {
	if values, ok := ctx.Value(preparedKey{}).(map[string]resolution); ok {
		v, exists := values[resolutionKey(id, source, c)]
		if !exists {
			return Target{}, Invalid("condition", "检测条件未预先核验")
		}
		return v.target, v.err
	}
	return s.provider.Resolve(ctx, id, source, c)
}

// Resolve external account/configuration dependencies before opening the short
// quality transaction. This also works with a one-connection database pool.
func (s *Service) prepareRoute(c *gin.Context) {
	path := c.FullPath()
	method := c.Request.Method
	needs := method == "POST" && (strings.HasSuffix(path, "/policies") || strings.HasSuffix(path, "/batch-update") || strings.HasSuffix(path, "/preview") || strings.HasSuffix(path, "/runs")) || method == "PUT" && strings.Contains(path, "/policies/")
	if !needs {
		c.Next()
		return
	}
	raw, err := body(c)
	if err != nil {
		writeError(c, err)
		c.Abort()
		return
	}
	c.Request.Body = ioBody(raw)
	var req struct {
		Items     []json.RawMessage `json:"items"`
		PolicyIDs []int64           `json:"policy_ids"`
	}
	_ = json.Unmarshal(raw, &req)
	candidates := []CreateItem{}
	ids := map[int64]bool{}
	for _, item := range req.Items {
		var v CreateItem
		_ = json.Unmarshal(item, &v)
		if v.AccountID > 0 {
			candidates = append(candidates, v)
		}
		var update UpdateItem
		_ = json.Unmarshal(item, &update)
		if update.PolicyID > 0 {
			ids[update.PolicyID] = true
		}
	}
	for _, id := range req.PolicyIDs {
		ids[id] = true
	}
	if method == "PUT" {
		id, e := parseID(c)
		if e == nil {
			ids[id] = true
		}
	}
	if len(ids) > 100 || len(candidates) > 100 {
		writeError(c, Invalid("items", "最多100项"))
		c.Abort()
		return
	}
	if len(ids) > 0 {
		_, err = s.Read(c.Request.Context(), func(t *Tx) (any, error) {
			for id := range ids {
				p, e := t.Policy(id, false)
				if e != nil {
					var qe *Error
					if errorsAs404(e, &qe) {
						continue
					}
					return nil, e
				}
				candidates = append(candidates, CreateItem{AccountID: p.AccountID, SourceGroupID: p.SourceGroupID, Condition: p.Condition})
			}
			return nil, nil
		})
		if err != nil {
			writeError(c, err)
			c.Abort()
			return
		}
	}
	values := map[string]resolution{}
	for _, item := range candidates {
		key := resolutionKey(item.AccountID, item.SourceGroupID, item.Condition)
		if _, exists := values[key]; exists {
			continue
		}
		t, e := s.provider.Resolve(c.Request.Context(), item.AccountID, item.SourceGroupID, item.Condition)
		values[key] = resolution{t, e}
	}
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), preparedKey{}, values))
	c.Next()
}
func ioBody(raw []byte) *bodyReader { return &bodyReader{Reader: bytes.NewReader(raw)} }

type bodyReader struct{ *bytes.Reader }

func (*bodyReader) Close() error { return nil }
func errorsAs404(e error, qe **Error) bool {
	v, ok := e.(*Error)
	if ok {
		*qe = v
	}
	return ok && v.Status == 404
}
