package common

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ConfigureDB extracts the *sql.DB from provider data, returning an error if
// the type is unexpected. Used by all resources and data sources in Configure().
func ConfigureDB(providerData any) (*sql.DB, error) {
	db, ok := providerData.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("expected *sql.DB, got: %T", providerData)
	}
	return db, nil
}

// IsSet returns true if a Terraform attribute value is neither null nor unknown.
func IsSet(val interface{ IsNull() bool; IsUnknown() bool }) bool {
	return !val.IsNull() && !val.IsUnknown()
}

// StringSetToSlice converts a types.Set of strings into a []string.
func StringSetToSlice(ctx context.Context, set types.Set) []string {
	if !IsSet(set) {
		return nil
	}
	var elems []types.String
	set.ElementsAs(ctx, &elems, false)
	result := make([]string, len(elems))
	for i, e := range elems {
		result[i] = e.ValueString()
	}
	return result
}

// StringListToSlice converts a types.List of strings into a []string.
func StringListToSlice(ctx context.Context, list types.List) []string {
	if !IsSet(list) {
		return nil
	}
	var elems []types.String
	list.ElementsAs(ctx, &elems, false)
	result := make([]string, len(elems))
	for i, e := range elems {
		result[i] = e.ValueString()
	}
	return result
}

// PrivilegesToSlice extracts privilege strings from a types.Set and uppercases them.
func PrivilegesToSlice(ctx context.Context, set types.Set) []string {
	raw := StringSetToSlice(ctx, set)
	for i, p := range raw {
		raw[i] = strings.ToUpper(p)
	}
	return raw
}
