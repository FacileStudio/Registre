package clients

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/schemas"
	"gorm.io/gorm"
)

// ErrUnknownResource is an MCP resource URI no enabled application declares.
var ErrUnknownResource = errors.New("clients: no application declares that resource")

// ByResource returns the enabled application that declares uri as its MCP
// resource. The match is exact, byte for byte, because RFC 8707 compares
// resource indicators as strings and a lenient match is how a token for one
// server ends up valid at a look-alike.
func (s *Service) ByResource(ctx context.Context, uri string) (schemas.Client, error) {
	if uri == "" {
		return schemas.Client{}, ErrUnknownResource
	}
	var row schemas.Client
	err := s.db.WithContext(ctx).Where("mcp_resource = ?", uri).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.Client{}, ErrUnknownResource
	}
	if err != nil {
		return schemas.Client{}, fmt.Errorf("clients: read the resource %s: %w", uri, err)
	}
	if !row.Enabled() {
		return schemas.Client{}, ErrUnknownResource
	}
	return row, nil
}

// Resources lists every enabled application that declares an MCP resource.
func (s *Service) Resources(ctx context.Context) ([]schemas.Client, error) {
	var rows []schemas.Client
	err := s.db.WithContext(ctx).
		Where("mcp_resource <> '' AND disabled_at IS NULL").
		Order("mcp_resource").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("clients: list the resources: %w", err)
	}
	return rows, nil
}

// LooksLikeResource reports whether value is shaped like an MCP resource URI
// rather than a client id: only an https URL can be one.
func LooksLikeResource(value string) bool {
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://")
}
