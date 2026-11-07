package login

import (
	"fmt"
	"net/http/httptest"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
)

func newRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }

func errBusyForTest() error { return fmt.Errorf("identity: verify: %w", credentials.ErrBusy) }
