package okteto

import (
	"context"
	"fmt"
	"strings"

	"github.com/machinebox/graphql"
)

// returns a filtered list of fields that are valid to query for the typeName given
func queryAvailableFields(ctx context.Context, client *graphql.Client, typeName string, required string) []string {
	var fields []string
	var response map[string]map[string][]map[string]string

	req := graphql.NewRequest(fmt.Sprintf(`query {
		__type(name:"%s") {
			fields {
				name
			}
		}
	}`, typeName))

	if err := client.Run(ctx, req, &response); err != nil {
		return fields
	}

	for _, v := range response["__type"]["fields"] {
		fields = append(fields, v["name"])
	}
	var validated []string
	for _, r := range strings.Split(required, ",") {
		for _, a := range fields {
			if r == a {
				validated = append(validated, r)
			}
		}
	}
	return validated

}
