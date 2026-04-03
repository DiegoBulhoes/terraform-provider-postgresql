package resource

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------
// readRole tests
// ---------------------------------------------------------------------------

func TestReadRole_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT oid, rolcanlogin").
		WillReturnError(sql.ErrNoRows)

	r := &roleResource{db: db}
	model := &roleResourceModel{Name: types.StringValue("ghost")}

	diags := r.readRole(context.Background(), model)
	if !diags.HasError() {
		t.Fatal("expected diagnostics to have an error, got none")
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "Role not found" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Role not found' diagnostic, got different errors")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadRole_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT oid, rolcanlogin").
		WillReturnError(fmt.Errorf("connection refused"))

	r := &roleResource{db: db}
	model := &roleResourceModel{Name: types.StringValue("testrole")}

	diags := r.readRole(context.Background(), model)
	if !diags.HasError() {
		t.Fatal("expected diagnostics to have an error, got none")
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "Error reading role" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error reading role' diagnostic")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadRole_MembershipQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	roleRows := sqlmock.NewRows([]string{
		"oid", "rolcanlogin", "rolsuper", "rolcreatedb",
		"rolcreaterole", "rolreplication", "rolconnlimit", "rolvaliduntil",
	}).AddRow(int64(12345), true, false, false, false, false, int64(-1), nil)

	mock.ExpectQuery("SELECT oid, rolcanlogin").
		WillReturnRows(roleRows)

	mock.ExpectQuery("SELECT r.rolname").
		WillReturnError(fmt.Errorf("membership query failed"))

	r := &roleResource{db: db}
	model := &roleResourceModel{Name: types.StringValue("testrole")}

	diags := r.readRole(context.Background(), model)
	if !diags.HasError() {
		t.Fatal("expected diagnostics to have an error, got none")
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "Error reading role memberships" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error reading role memberships' diagnostic")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadRole_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	roleRows := sqlmock.NewRows([]string{
		"oid", "rolcanlogin", "rolsuper", "rolcreatedb",
		"rolcreaterole", "rolreplication", "rolconnlimit", "rolvaliduntil",
	}).AddRow(int64(99), true, true, false, true, false, int64(10), "2026-12-31T23:59:59Z")

	mock.ExpectQuery("SELECT oid, rolcanlogin").
		WillReturnRows(roleRows)

	memberRows := sqlmock.NewRows([]string{"rolname"}).
		AddRow("admin").
		AddRow("readonly")

	mock.ExpectQuery("SELECT r.rolname").
		WillReturnRows(memberRows)

	r := &roleResource{db: db}
	model := &roleResourceModel{Name: types.StringValue("testrole")}

	diags := r.readRole(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if model.OID.ValueInt64() != 99 {
		t.Errorf("expected OID 99, got %d", model.OID.ValueInt64())
	}
	if !model.Login.ValueBool() {
		t.Error("expected Login to be true")
	}
	if !model.Superuser.ValueBool() {
		t.Error("expected Superuser to be true")
	}
	if model.CreateDatabase.ValueBool() {
		t.Error("expected CreateDatabase to be false")
	}
	if !model.CreateRole.ValueBool() {
		t.Error("expected CreateRole to be true")
	}
	if model.Replication.ValueBool() {
		t.Error("expected Replication to be false")
	}
	if model.ConnectionLimit.ValueInt64() != 10 {
		t.Errorf("expected ConnectionLimit 10, got %d", model.ConnectionLimit.ValueInt64())
	}
	if model.ValidUntil.ValueString() != "2026-12-31T23:59:59Z" {
		t.Errorf("expected ValidUntil '2026-12-31T23:59:59Z', got %q", model.ValidUntil.ValueString())
	}

	// Check membership list has 2 elements
	elems := model.Roles.Elements()
	if len(elems) != 2 {
		t.Fatalf("expected 2 role memberships, got %d", len(elems))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadRole_NullValidUntil(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	roleRows := sqlmock.NewRows([]string{
		"oid", "rolcanlogin", "rolsuper", "rolcreatedb",
		"rolcreaterole", "rolreplication", "rolconnlimit", "rolvaliduntil",
	}).AddRow(int64(42), false, false, false, false, false, int64(-1), nil)

	mock.ExpectQuery("SELECT oid, rolcanlogin").
		WillReturnRows(roleRows)

	memberRows := sqlmock.NewRows([]string{"rolname"})
	mock.ExpectQuery("SELECT r.rolname").
		WillReturnRows(memberRows)

	r := &roleResource{db: db}
	model := &roleResourceModel{Name: types.StringValue("norole")}

	diags := r.readRole(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if !model.ValidUntil.IsNull() {
		t.Errorf("expected ValidUntil to be null, got %q", model.ValidUntil.ValueString())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// readDatabase tests
// ---------------------------------------------------------------------------

func TestReadDatabase_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT").
		WithArgs("testdb").
		WillReturnError(sql.ErrNoRows)

	r := &databaseResource{db: db}
	model := &databaseResourceModel{Name: types.StringValue("testdb")}

	diags := r.readDatabase(context.Background(), model)
	if !diags.HasError() {
		t.Fatal("expected diagnostics to have an error, got none")
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "Database not found" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Database not found' diagnostic")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadDatabase_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT").
		WithArgs("testdb").
		WillReturnError(fmt.Errorf("disk I/O error"))

	r := &databaseResource{db: db}
	model := &databaseResourceModel{Name: types.StringValue("testdb")}

	diags := r.readDatabase(context.Background(), model)
	if !diags.HasError() {
		t.Fatal("expected diagnostics to have an error, got none")
	}

	found := false
	for _, d := range diags {
		if d.Summary() == "Error reading database" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Error reading database' diagnostic")
		for _, d := range diags {
			t.Logf("  diagnostic: %s - %s", d.Summary(), d.Detail())
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadDatabase_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"oid", "owner", "encoding", "lc_collate", "lc_ctype",
		"allow_connections", "connection_limit", "is_template", "tablespace_name",
	}).AddRow(
		int64(16384), "postgres", "UTF8", "en_US.UTF-8", "en_US.UTF-8",
		true, int64(-1), false, "pg_default",
	)

	mock.ExpectQuery("SELECT").
		WithArgs("mydb").
		WillReturnRows(rows)

	r := &databaseResource{db: db}
	model := &databaseResourceModel{
		Name:     types.StringValue("mydb"),
		Template: types.StringNull(),
	}

	diags := r.readDatabase(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if model.OID.ValueInt64() != 16384 {
		t.Errorf("expected OID 16384, got %d", model.OID.ValueInt64())
	}
	if model.Owner.ValueString() != "postgres" {
		t.Errorf("expected owner 'postgres', got %q", model.Owner.ValueString())
	}
	if model.Encoding.ValueString() != "UTF8" {
		t.Errorf("expected encoding 'UTF8', got %q", model.Encoding.ValueString())
	}
	if model.LcCollate.ValueString() != "en_US.UTF-8" {
		t.Errorf("expected lc_collate 'en_US.UTF-8', got %q", model.LcCollate.ValueString())
	}
	if model.AllowConnections.ValueBool() != true {
		t.Error("expected AllowConnections to be true")
	}
	if model.ConnectionLimit.ValueInt64() != -1 {
		t.Errorf("expected ConnectionLimit -1, got %d", model.ConnectionLimit.ValueInt64())
	}
	if model.IsTemplate.ValueBool() != false {
		t.Error("expected IsTemplate to be false")
	}
	if model.TablespaceName.ValueString() != "pg_default" {
		t.Errorf("expected tablespace 'pg_default', got %q", model.TablespaceName.ValueString())
	}
	// Template should be set to default when null
	if model.Template.ValueString() != "template0" {
		t.Errorf("expected template 'template0', got %q", model.Template.ValueString())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadDatabase_PreservesExistingTemplate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"oid", "owner", "encoding", "lc_collate", "lc_ctype",
		"allow_connections", "connection_limit", "is_template", "tablespace_name",
	}).AddRow(
		int64(16384), "postgres", "UTF8", "en_US.UTF-8", "en_US.UTF-8",
		true, int64(-1), false, "pg_default",
	)

	mock.ExpectQuery("SELECT").
		WithArgs("mydb").
		WillReturnRows(rows)

	r := &databaseResource{db: db}
	model := &databaseResourceModel{
		Name:     types.StringValue("mydb"),
		Template: types.StringValue("template1"),
	}

	diags := r.readDatabase(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	// Template should be preserved when already set
	if model.Template.ValueString() != "template1" {
		t.Errorf("expected template 'template1' to be preserved, got %q", model.Template.ValueString())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// buildRoleOptions tests
// ---------------------------------------------------------------------------

func TestBuildRoleOptions_AllDefaults(t *testing.T) {
	r := &roleResource{}
	model := &roleResourceModel{
		Login:           types.BoolValue(false),
		Superuser:       types.BoolValue(false),
		CreateDatabase:  types.BoolValue(false),
		CreateRole:      types.BoolValue(false),
		Replication:     types.BoolValue(false),
		ConnectionLimit: types.Int64Value(-1),
		Password:        types.StringNull(),
		ValidUntil:      types.StringNull(),
	}

	result := r.buildRoleOptions(context.Background(), model)

	expected := []string{"NOLOGIN", "NOSUPERUSER", "NOCREATEDB", "NOCREATEROLE", "NOREPLICATION", "CONNECTION LIMIT -1"}
	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("expected %q in result %q", exp, result)
		}
	}

	if strings.Contains(result, "PASSWORD") {
		t.Error("did not expect PASSWORD in result when password is null")
	}
	if strings.Contains(result, "VALID UNTIL") {
		t.Error("did not expect VALID UNTIL in result when valid_until is null")
	}
}

func TestBuildRoleOptions_AllEnabled(t *testing.T) {
	r := &roleResource{}
	model := &roleResourceModel{
		Login:           types.BoolValue(true),
		Superuser:       types.BoolValue(true),
		CreateDatabase:  types.BoolValue(true),
		CreateRole:      types.BoolValue(true),
		Replication:     types.BoolValue(true),
		ConnectionLimit: types.Int64Value(50),
		Password:        types.StringValue("s3cret"),
		ValidUntil:      types.StringValue("2026-12-31T23:59:59Z"),
	}

	result := r.buildRoleOptions(context.Background(), model)

	expected := []string{"LOGIN", "SUPERUSER", "CREATEDB", "CREATEROLE", "REPLICATION", "CONNECTION LIMIT 50"}
	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("expected %q in result %q", exp, result)
		}
	}

	if !strings.Contains(result, "PASSWORD") {
		t.Error("expected PASSWORD in result")
	}
	if !strings.Contains(result, "VALID UNTIL") {
		t.Error("expected VALID UNTIL in result")
	}

	// Ensure the result starts with " WITH "
	if !strings.HasPrefix(result, " WITH ") {
		t.Errorf("expected result to start with ' WITH ', got %q", result)
	}
}

func TestBuildRoleOptions_PasswordIncluded(t *testing.T) {
	r := &roleResource{}
	model := &roleResourceModel{
		Login:           types.BoolValue(false),
		Superuser:       types.BoolValue(false),
		CreateDatabase:  types.BoolValue(false),
		CreateRole:      types.BoolValue(false),
		Replication:     types.BoolValue(false),
		ConnectionLimit: types.Int64Value(-1),
		Password:        types.StringValue("hunter2"),
		ValidUntil:      types.StringNull(),
	}

	result := r.buildRoleOptions(context.Background(), model)

	if !strings.Contains(result, "PASSWORD") {
		t.Errorf("expected PASSWORD in result %q", result)
	}
	// Password should be quoted as a literal (single quotes)
	if !strings.Contains(result, "'hunter2'") {
		t.Errorf("expected quoted password in result %q", result)
	}
}

// ---------------------------------------------------------------------------
// diffRoles tests
// ---------------------------------------------------------------------------

func TestDiffRoles_NoChange(t *testing.T) {
	toGrant, toRevoke := diffRoles([]string{"a", "b"}, []string{"a", "b"})
	if len(toGrant) != 0 {
		t.Errorf("expected no grants, got %v", toGrant)
	}
	if len(toRevoke) != 0 {
		t.Errorf("expected no revokes, got %v", toRevoke)
	}
}

func TestDiffRoles_AddNew(t *testing.T) {
	toGrant, toRevoke := diffRoles([]string{"a"}, []string{"a", "b", "c"})
	if len(toGrant) != 2 {
		t.Fatalf("expected 2 grants, got %v", toGrant)
	}
	if len(toRevoke) != 0 {
		t.Errorf("expected no revokes, got %v", toRevoke)
	}
}

func TestDiffRoles_RemoveOld(t *testing.T) {
	toGrant, toRevoke := diffRoles([]string{"a", "b", "c"}, []string{"a"})
	if len(toGrant) != 0 {
		t.Errorf("expected no grants, got %v", toGrant)
	}
	if len(toRevoke) != 2 {
		t.Fatalf("expected 2 revokes, got %v", toRevoke)
	}
}

func TestDiffRoles_Mixed(t *testing.T) {
	toGrant, toRevoke := diffRoles([]string{"a", "b"}, []string{"b", "c"})
	if len(toGrant) != 1 || toGrant[0] != "c" {
		t.Errorf("expected grant ['c'], got %v", toGrant)
	}
	if len(toRevoke) != 1 || toRevoke[0] != "a" {
		t.Errorf("expected revoke ['a'], got %v", toRevoke)
	}
}

func TestDiffRoles_BothEmpty(t *testing.T) {
	toGrant, toRevoke := diffRoles(nil, nil)
	if len(toGrant) != 0 {
		t.Errorf("expected no grants, got %v", toGrant)
	}
	if len(toRevoke) != 0 {
		t.Errorf("expected no revokes, got %v", toRevoke)
	}
}

// ---------------------------------------------------------------------------
// readPrivileges tests
// ---------------------------------------------------------------------------

func TestReadPrivileges_Database_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"privilege_type", "is_grantable"}).
		AddRow("CONNECT", true).
		AddRow("CREATE", true)

	mock.ExpectQuery("SELECT privilege_type, is_grantable").
		WithArgs("mydb", "myrole").
		WillReturnRows(rows)

	r := &grantResource{db: db}
	privs, allGrantable, err := r.readPrivileges(context.Background(), "myrole", "database", "mydb", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allGrantable {
		t.Error("expected allGrantable to be true")
	}
	if len(privs) != 2 {
		t.Fatalf("expected 2 privileges, got %d", len(privs))
	}
	if privs[0] != "CONNECT" || privs[1] != "CREATE" {
		t.Errorf("expected [CONNECT, CREATE], got %v", privs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadPrivileges_Database_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT privilege_type, is_grantable").
		WithArgs("mydb", "myrole").
		WillReturnError(fmt.Errorf("connection refused"))

	r := &grantResource{db: db}
	privs, allGrantable, err := r.readPrivileges(context.Background(), "myrole", "database", "mydb", "", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected 'connection refused' in error, got %q", err.Error())
	}
	if privs != nil {
		t.Errorf("expected nil privileges, got %v", privs)
	}
	if allGrantable {
		t.Error("expected allGrantable to be false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadPrivileges_Database_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"privilege_type", "is_grantable"})

	mock.ExpectQuery("SELECT privilege_type, is_grantable").
		WithArgs("mydb", "myrole").
		WillReturnRows(rows)

	r := &grantResource{db: db}
	privs, allGrantable, err := r.readPrivileges(context.Background(), "myrole", "database", "mydb", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allGrantable {
		t.Error("expected allGrantable to be false when no rows returned")
	}
	if privs != nil {
		t.Errorf("expected nil privileges, got %v", privs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadPrivileges_Schema_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"privilege_type", "is_grantable"}).
		AddRow("USAGE", true).
		AddRow("CREATE", false)

	mock.ExpectQuery("SELECT privilege_type, is_grantable").
		WithArgs("public", "myrole").
		WillReturnRows(rows)

	r := &grantResource{db: db}
	privs, allGrantable, err := r.readPrivileges(context.Background(), "myrole", "schema", "", "public", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allGrantable {
		t.Error("expected allGrantable to be false when one privilege is not grantable")
	}
	if len(privs) != 2 {
		t.Fatalf("expected 2 privileges, got %d", len(privs))
	}
	if privs[0] != "USAGE" || privs[1] != "CREATE" {
		t.Errorf("expected [USAGE, CREATE], got %v", privs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadPrivileges_Table_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"privilege_type", "is_grantable"}).
		AddRow("SELECT", true).
		AddRow("INSERT", true)

	mock.ExpectQuery("SELECT privilege_type, is_grantable").
		WithArgs("public", "my_table", "myrole").
		WillReturnRows(rows)

	r := &grantResource{db: db}
	privs, allGrantable, err := r.readPrivileges(context.Background(), "myrole", "table", "", "public", []string{"my_table"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allGrantable {
		t.Error("expected allGrantable to be true")
	}
	if len(privs) != 2 {
		t.Fatalf("expected 2 privileges, got %d", len(privs))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadPrivileges_Sequence_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"privilege_type", "is_grantable"}).
		AddRow("USAGE", true)

	mock.ExpectQuery("SELECT privilege_type, is_grantable").
		WithArgs("public", "my_seq", "myrole").
		WillReturnRows(rows)

	r := &grantResource{db: db}
	privs, allGrantable, err := r.readPrivileges(context.Background(), "myrole", "sequence", "", "public", []string{"my_seq"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allGrantable {
		t.Error("expected allGrantable to be true")
	}
	if len(privs) != 1 || privs[0] != "USAGE" {
		t.Errorf("expected [USAGE], got %v", privs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadPrivileges_Function_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"privilege_type", "is_grantable"}).
		AddRow("EXECUTE", false)

	mock.ExpectQuery("SELECT privilege_type, is_grantable").
		WithArgs("public", "my_func", "myrole").
		WillReturnRows(rows)

	r := &grantResource{db: db}
	privs, allGrantable, err := r.readPrivileges(context.Background(), "myrole", "function", "", "public", []string{"my_func"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allGrantable {
		t.Error("expected allGrantable to be false")
	}
	if len(privs) != 1 || privs[0] != "EXECUTE" {
		t.Errorf("expected [EXECUTE], got %v", privs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestReadPrivileges_UnsupportedType(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	r := &grantResource{db: db}
	privs, allGrantable, err := r.readPrivileges(context.Background(), "myrole", "invalid", "", "", nil)
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported object type") {
		t.Errorf("expected 'unsupported object type' in error, got %q", err.Error())
	}
	if privs != nil {
		t.Errorf("expected nil privileges, got %v", privs)
	}
	if allGrantable {
		t.Error("expected allGrantable to be false")
	}
}

func TestReadPrivileges_ScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// Return rows with wrong column types: two strings instead of string+bool
	rows := sqlmock.NewRows([]string{"privilege_type", "is_grantable"}).
		AddRow("SELECT", "not_a_bool")

	mock.ExpectQuery("SELECT privilege_type, is_grantable").
		WithArgs("mydb", "myrole").
		WillReturnRows(rows)

	r := &grantResource{db: db}
	_, _, err = r.readPrivileges(context.Background(), "myrole", "database", "mydb", "", nil)
	if err == nil {
		t.Fatal("expected scan error, got nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// buildGrantStatements tests
// ---------------------------------------------------------------------------

func TestBuildGrantStatements_Database(t *testing.T) {
	stmts := buildGrantStatements("CONNECT, CREATE", "database", "mydb", "", "myrole", nil, "")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "GRANT CONNECT, CREATE ON DATABASE") {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `"mydb"`) {
		t.Errorf("expected quoted database name in statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `TO "myrole"`) {
		t.Errorf("expected TO quoted role in statement: %s", stmts[0])
	}
}

func TestBuildGrantStatements_Schema(t *testing.T) {
	stmts := buildGrantStatements("USAGE, CREATE", "schema", "", "myschema", "myrole", nil, "")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "GRANT USAGE, CREATE ON SCHEMA") {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `"myschema"`) {
		t.Errorf("expected quoted schema name in statement: %s", stmts[0])
	}
}

func TestBuildGrantStatements_TableAll(t *testing.T) {
	stmts := buildGrantStatements("SELECT", "table", "", "myschema", "myrole", nil, "")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ALL TABLES IN SCHEMA") {
		t.Errorf("expected ALL TABLES IN SCHEMA in statement: %s", stmts[0])
	}
}

func TestBuildGrantStatements_TableSpecific(t *testing.T) {
	stmts := buildGrantStatements("SELECT", "table", "", "myschema", "myrole", []string{"t1", "t2"}, "")
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `ON TABLE "myschema"."t1"`) {
		t.Errorf("expected first statement to reference t1: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], `ON TABLE "myschema"."t2"`) {
		t.Errorf("expected second statement to reference t2: %s", stmts[1])
	}
}

func TestBuildGrantStatements_WithGrantOption(t *testing.T) {
	stmts := buildGrantStatements("SELECT", "database", "mydb", "", "myrole", nil, " WITH GRANT OPTION")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.HasSuffix(stmts[0], " WITH GRANT OPTION") {
		t.Errorf("expected statement to end with WITH GRANT OPTION: %s", stmts[0])
	}
}

func TestBuildGrantStatements_SequenceAll(t *testing.T) {
	stmts := buildGrantStatements("USAGE", "sequence", "", "myschema", "myrole", nil, "")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ALL SEQUENCES IN SCHEMA") {
		t.Errorf("expected ALL SEQUENCES IN SCHEMA in statement: %s", stmts[0])
	}
}

func TestBuildGrantStatements_SequenceSpecific(t *testing.T) {
	stmts := buildGrantStatements("USAGE", "sequence", "", "myschema", "myrole", []string{"seq1", "seq2"}, "")
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `ON SEQUENCE "myschema"."seq1"`) {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], `ON SEQUENCE "myschema"."seq2"`) {
		t.Errorf("unexpected statement: %s", stmts[1])
	}
}

func TestBuildGrantStatements_FunctionAll(t *testing.T) {
	stmts := buildGrantStatements("EXECUTE", "function", "", "myschema", "myrole", nil, "")
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ALL FUNCTIONS IN SCHEMA") {
		t.Errorf("expected ALL FUNCTIONS IN SCHEMA in statement: %s", stmts[0])
	}
}

func TestBuildGrantStatements_FunctionSpecific(t *testing.T) {
	stmts := buildGrantStatements("EXECUTE", "function", "", "myschema", "myrole", []string{"fn1", "fn2"}, "")
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `ON FUNCTION "myschema"."fn1"`) {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], `ON FUNCTION "myschema"."fn2"`) {
		t.Errorf("unexpected statement: %s", stmts[1])
	}
}

// ---------------------------------------------------------------------------
// buildRevokeStatements tests
// ---------------------------------------------------------------------------

func TestBuildRevokeStatements_Database(t *testing.T) {
	stmts := buildRevokeStatements("database", "mydb", "", "myrole", nil)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "REVOKE ALL PRIVILEGES ON DATABASE") {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `FROM "myrole"`) {
		t.Errorf("expected FROM quoted role in statement: %s", stmts[0])
	}
}

func TestBuildRevokeStatements_Schema(t *testing.T) {
	stmts := buildRevokeStatements("schema", "", "myschema", "myrole", nil)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "REVOKE ALL PRIVILEGES ON SCHEMA") {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `"myschema"`) {
		t.Errorf("expected quoted schema in statement: %s", stmts[0])
	}
}

func TestBuildRevokeStatements_TableAll(t *testing.T) {
	stmts := buildRevokeStatements("table", "", "myschema", "myrole", nil)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ALL TABLES IN SCHEMA") {
		t.Errorf("expected ALL TABLES IN SCHEMA in statement: %s", stmts[0])
	}
}

func TestBuildRevokeStatements_TableSpecific(t *testing.T) {
	stmts := buildRevokeStatements("table", "", "myschema", "myrole", []string{"t1", "t2"})
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `ON TABLE "myschema"."t1"`) {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], `ON TABLE "myschema"."t2"`) {
		t.Errorf("unexpected statement: %s", stmts[1])
	}
}

func TestBuildRevokeStatements_SequenceAll(t *testing.T) {
	stmts := buildRevokeStatements("sequence", "", "myschema", "myrole", nil)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ALL SEQUENCES IN SCHEMA") {
		t.Errorf("expected ALL SEQUENCES IN SCHEMA in statement: %s", stmts[0])
	}
}

func TestBuildRevokeStatements_SequenceSpecific(t *testing.T) {
	stmts := buildRevokeStatements("sequence", "", "myschema", "myrole", []string{"seq1"})
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `ON SEQUENCE "myschema"."seq1"`) {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
}

func TestBuildRevokeStatements_FunctionAll(t *testing.T) {
	stmts := buildRevokeStatements("function", "", "myschema", "myrole", nil)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "ALL FUNCTIONS IN SCHEMA") {
		t.Errorf("expected ALL FUNCTIONS IN SCHEMA in statement: %s", stmts[0])
	}
}

func TestBuildRevokeStatements_FunctionSpecific(t *testing.T) {
	stmts := buildRevokeStatements("function", "", "myschema", "myrole", []string{"fn1", "fn2"})
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `ON FUNCTION "myschema"."fn1"`) {
		t.Errorf("unexpected statement: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], `ON FUNCTION "myschema"."fn2"`) {
		t.Errorf("unexpected statement: %s", stmts[1])
	}
}

// ---------------------------------------------------------------------------
// defaultPrivilegesResource helper tests
// ---------------------------------------------------------------------------

func TestDefaultPrivileges_BuildGrantSQL_WithSchema(t *testing.T) {
	r := &defaultPrivilegesResource{}
	result := r.buildGrantSQL("owner1", "grantee1", types.StringValue("myschema"), []string{"SELECT", "INSERT"}, "TABLES")

	expected := `ALTER DEFAULT PRIVILEGES FOR ROLE "owner1" IN SCHEMA "myschema" GRANT SELECT, INSERT ON TABLES TO "grantee1"`
	if result != expected {
		t.Errorf("unexpected SQL:\n  got:  %s\n  want: %s", result, expected)
	}
}

func TestDefaultPrivileges_BuildGrantSQL_NoSchema(t *testing.T) {
	r := &defaultPrivilegesResource{}
	result := r.buildGrantSQL("owner1", "grantee1", types.StringNull(), []string{"USAGE"}, "SEQUENCES")

	expected := `ALTER DEFAULT PRIVILEGES FOR ROLE "owner1" GRANT USAGE ON SEQUENCES TO "grantee1"`
	if result != expected {
		t.Errorf("unexpected SQL:\n  got:  %s\n  want: %s", result, expected)
	}
	if strings.Contains(result, "IN SCHEMA") {
		t.Error("expected no IN SCHEMA clause when schema is null")
	}
}

func TestDefaultPrivileges_BuildRevokeAllSQL_WithSchema(t *testing.T) {
	r := &defaultPrivilegesResource{}
	result := r.buildRevokeAllSQL("owner1", "grantee1", types.StringValue("myschema"), "TABLES")

	expected := `ALTER DEFAULT PRIVILEGES FOR ROLE "owner1" IN SCHEMA "myschema" REVOKE ALL ON TABLES FROM "grantee1"`
	if result != expected {
		t.Errorf("unexpected SQL:\n  got:  %s\n  want: %s", result, expected)
	}
}

func TestDefaultPrivileges_BuildRevokeAllSQL_NoSchema(t *testing.T) {
	r := &defaultPrivilegesResource{}
	result := r.buildRevokeAllSQL("owner1", "grantee1", types.StringNull(), "FUNCTIONS")

	expected := `ALTER DEFAULT PRIVILEGES FOR ROLE "owner1" REVOKE ALL ON FUNCTIONS FROM "grantee1"`
	if result != expected {
		t.Errorf("unexpected SQL:\n  got:  %s\n  want: %s", result, expected)
	}
	if strings.Contains(result, "IN SCHEMA") {
		t.Error("expected no IN SCHEMA clause when schema is null")
	}
}

func TestDefaultPrivileges_CompositeID_WithSchema(t *testing.T) {
	r := &defaultPrivilegesResource{}
	data := defaultPrivilegesResourceModel{
		Owner:      types.StringValue("o"),
		Role:       types.StringValue("r"),
		Database:   types.StringValue("d"),
		Schema:     types.StringValue("s"),
		ObjectType: types.StringValue("table"),
	}
	id := r.compositeID(data)
	if id != "o_r_d_s_table" {
		t.Errorf("expected 'o_r_d_s_table', got %q", id)
	}
}

func TestDefaultPrivileges_CompositeID_NoSchema(t *testing.T) {
	r := &defaultPrivilegesResource{}
	data := defaultPrivilegesResourceModel{
		Owner:      types.StringValue("o"),
		Role:       types.StringValue("r"),
		Database:   types.StringValue("d"),
		Schema:     types.StringNull(),
		ObjectType: types.StringValue("table"),
	}
	id := r.compositeID(data)
	if id != "o_r_d__table" {
		t.Errorf("expected 'o_r_d__table', got %q", id)
	}
}
