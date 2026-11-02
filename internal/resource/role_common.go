package resource

import (
	"context"
	"database/sql"
	"errors"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// roleAttributes points at the fields postgresql_role and postgresql_user share.
type roleAttributes struct {
	OID             *types.Int64
	Superuser       *types.Bool
	CreateDatabase  *types.Bool
	CreateRole      *types.Bool
	Replication     *types.Bool
	ConnectionLimit *types.Int64
}

// roleRow is the part of pg_roles both resources read.
type roleRow struct {
	OID         int64
	CanLogin    bool
	Superuser   bool
	CreateDB    bool
	CreateRole  bool
	Replication bool
	ConnLimit   int64
	ValidUntil  sql.NullString
}

const roleRowQuery = `SELECT oid, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole,
	rolreplication, rolconnlimit,
	CASE WHEN rolvaliduntil = 'infinity' THEN NULL ELSE rolvaliduntil END
	FROM pg_catalog.pg_roles WHERE rolname = $1`

// readRoleRow reads a role from pg_roles; found is false when it does not exist.
func readRoleRow(ctx context.Context, db common.DBTX, name string) (roleRow, bool, error) {
	var row roleRow
	err := db.QueryRowContext(ctx, roleRowQuery, name).Scan(
		&row.OID, &row.CanLogin, &row.Superuser, &row.CreateDB, &row.CreateRole,
		&row.Replication, &row.ConnLimit, &row.ValidUntil,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return row, false, nil
	}
	return row, true, err
}

func (a roleAttributes) set(row roleRow) {
	*a.OID = types.Int64Value(row.OID)
	*a.Superuser = types.BoolValue(row.Superuser)
	*a.CreateDatabase = types.BoolValue(row.CreateDB)
	*a.CreateRole = types.BoolValue(row.CreateRole)
	*a.Replication = types.BoolValue(row.Replication)
	*a.ConnectionLimit = types.Int64Value(row.ConnLimit)
}

// options returns the WITH options of the shared attributes; prev is nil on create.
func (a roleAttributes) options(prev *roleAttributes) common.RoleOptions {
	state := func(v *types.Bool) types.Bool {
		if prev == nil {
			return types.BoolNull()
		}
		return *v
	}
	var p roleAttributes
	if prev != nil {
		p = *prev
	}
	return common.RoleOptions{
		ConnectionLimit: a.ConnectionLimit.ValueInt64(),
		Superuser:       common.FlagChange(*a.Superuser, state(p.Superuser)),
		CreateDatabase:  common.FlagChange(*a.CreateDatabase, state(p.CreateDatabase)),
		CreateRole:      common.FlagChange(*a.CreateRole, state(p.CreateRole)),
		Replication:     common.FlagChange(*a.Replication, state(p.Replication)),
	}
}

func (m *RoleResourceModel) attributes() roleAttributes {
	return roleAttributes{&m.OID, &m.Superuser, &m.CreateDatabase, &m.CreateRole, &m.Replication, &m.ConnectionLimit}
}

func (m *UserResourceModel) attributes() roleAttributes {
	return roleAttributes{&m.OID, &m.Superuser, &m.CreateDatabase, &m.CreateRole, &m.Replication, &m.ConnectionLimit}
}
