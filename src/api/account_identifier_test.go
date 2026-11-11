package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bbernhard/signal-cli-rest-api/utils"
	"github.com/gin-gonic/gin"
)

func TestAccountIdentifierFromParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("e164", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Params = gin.Params{{Key: "number", Value: "%2B12025550123"}}
		typ, value, err := accountIdentifierFromParam(c)
		if err != nil {
			t.Fatal(err)
		}
		if typ != utils.IdentifierE164 || value != "+12025550123" {
			t.Fatalf("got %v %q", typ, value)
		}
		flag, selected := utils.SignalCliAccountSelector(typ, value)
		if flag != "-a" || selected != "+12025550123" {
			t.Fatalf("selector %s %s", flag, selected)
		}
	})

	t.Run("aci", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Params = gin.Params{{Key: "number", Value: "AAAAAAAA-BBBB-4CCC-8DDD-EEEEEEEEEEEE"}}
		typ, value, err := accountIdentifierFromParam(c)
		if err != nil {
			t.Fatal(err)
		}
		if typ != utils.IdentifierACI || value != "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee" {
			t.Fatalf("got %v %q", typ, value)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "number", Value: "not-an-account"}}
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/register/not-an-account", strings.NewReader("{}"))
		NewApi(nil).RegisterNumber(c)
		if w.Code != 400 {
			t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "invalid account identifier") {
			t.Fatalf("body = %s", w.Body.String())
		}
	})
}

// An ACI has no phone number to send a code to, so it can only be registered with a recovery key and never verified.
func TestRegisterByACIRequiresRecoveryKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	aci := gin.Params{{Key: "number", Value: "11111111-1111-4111-8111-111111111111"}, {Key: "token", Value: "123456"}}

	for name, handler := range map[string]gin.HandlerFunc{"register": NewApi(nil).RegisterNumber, "verify": NewApi(nil).VerifyRegisteredNumber} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = aci
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
		handler(c)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "recovery_key") {
			t.Fatalf("%s: status = %d, body %s", name, w.Code, w.Body.String())
		}
	}
}
