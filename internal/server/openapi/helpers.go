package openapi

func jsonRequestBody(description string, schemaName string, required bool) *OpenAPIRequestBody {
	return &OpenAPIRequestBody{
		Description: description,
		Required:    required,
		Content: map[string]OpenAPIMediaType{
			contentTypeJSON: {
				Schema: schemaRef(schemaName),
			},
		},
	}
}

func jsonResponse(description string, schemaName string) OpenAPIResponse {
	return OpenAPIResponse{
		Description: description,
		Content: map[string]OpenAPIMediaType{
			contentTypeJSON: {
				Schema: schemaRef(schemaName),
			},
		},
	}
}

func schemaRef(schemaName string) OpenAPISchema {
	return OpenAPISchema{
		Ref: openAPIRefPrefix + schemaName,
	}
}

func nullableSchema(schema OpenAPISchema) OpenAPISchema {
	schema.Nullable = true

	return schema
}

func objectSchema(
	description string,
	properties map[string]OpenAPISchema,
	required []string,
) OpenAPISchema {
	return OpenAPISchema{
		Type:        "object",
		Description: description,
		Properties:  properties,
		Required:    required,
	}
}

func objectSchemaWithAdditionalProperties(
	description string,
	additionalProperties *OpenAPISchema,
) OpenAPISchema {
	return OpenAPISchema{
		Type:                 "object",
		Description:          description,
		AdditionalProperties: additionalProperties,
	}
}

func stringSchema(description string) OpenAPISchema {
	return OpenAPISchema{
		Type:        "string",
		Description: description,
	}
}

func stringFormatSchema(format string, description string) OpenAPISchema {
	return OpenAPISchema{
		Type:        "string",
		Format:      format,
		Description: description,
	}
}

func enumStringSchema(description string, values []string) OpenAPISchema {
	return OpenAPISchema{
		Type:        "string",
		Description: description,
		Enum:        values,
	}
}

func boolSchema(description string) OpenAPISchema {
	return OpenAPISchema{
		Type:        "boolean",
		Description: description,
	}
}

func integerSchema(description string) OpenAPISchema {
	return OpenAPISchema{
		Type:        "integer",
		Description: description,
	}
}

func numberSchema(format string, description string) OpenAPISchema {
	return OpenAPISchema{
		Type:        "number",
		Format:      format,
		Description: description,
	}
}

func arraySchema(description string, itemSchema OpenAPISchema) OpenAPISchema {
	return OpenAPISchema{
		Type:        "array",
		Description: description,
		Items:       &itemSchema,
	}
}

func arraySchemaWithMaxItems(
	description string,
	itemSchema OpenAPISchema,
	maxItems int,
) OpenAPISchema {
	schema := arraySchema(description, itemSchema)
	schema.MaxItems = ptr(maxItems)

	return schema
}

func ptr[T any](value T) *T {
	return &value
}
