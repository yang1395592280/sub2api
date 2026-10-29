package accountquality

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCandyDetailHTTPRespectsResponseAccess(t *testing.T) {
	for _, suite := range []string{"candy", "dual"} {
		for _, access := range []string{"included", "omitted", "expired"} {
			t.Run(suite+"/"+access, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				defer db.Close()
				now := time.Now()
				expires := now.Add(time.Hour)
				if access == "expired" {
					expires = now.Add(-time.Second)
				}
				condition := Condition{BenchmarkID: suite, BenchmarkVersion: "2", ScorerVersion: "answer-explanation-v2"}
				if suite == "dual" {
					condition.ScorerVersion = "dual-visual-v2"
				}
				text := `{"answer":21,"explanation":"private-solution-summary"}`
				result := ScoreCandy(text, true, condition)
				if suite == "dual" {
					result = Combine(Check(result), nil, nil, &Evidence{Candy: text, SVG: "private-artwork"})
				}
				raw, err := EncodeEvidence(result)
				require.NoError(t, err)
				metadata, _ := json.Marshal(result)
				snapshot, _ := json.Marshal(Policy{Condition: condition})
				settings, _ := json.Marshal(DefaultSettings())
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT value, NOW() FROM settings WHERE key=$1`)).WithArgs(SettingsKey).WillReturnRows(sqlmock.NewRows([]string{"value", "now"}).AddRow(string(settings), now))
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT ` + runColumns + ` FROM account_quality_test_runs WHERE id=$1`)).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows(strings.Split(runColumns, ",")).AddRow(1, "batch", 1, 42, snapshot, "manual", 1, "completed", "passed", "correct_answer", nil, nil, nil, false, now, now, now, now, now, metadata, raw, expires, false, ""))
				mock.ExpectCommit()
				router := gin.New()
				NewService(NewStore(db, "Asia/Shanghai"), nil).Register(router.Group("/admin"))
				url := "/admin/account-quality/runs/1"
				if access != "omitted" {
					url += "?include_response=true"
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, url, nil))
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				require.Contains(t, response.Body.String(), `"question"`)
				if access == "included" {
					require.Contains(t, response.Body.String(), `"explanation":"private-solution-summary"`)
				} else {
					require.NotContains(t, response.Body.String(), "private-solution-summary")
					require.NotContains(t, response.Body.String(), "private-artwork")
				}
				require.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}
