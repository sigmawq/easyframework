package easyframework

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"
)

func TypeToMarkdown(sb *strings.Builder, value any) {
	typeof := reflect.TypeOf(value)

	_TypeToMarkdown(typeof, sb, 0, false)
	sb.WriteString("\n")
}

func WriteWithIndent(sb *strings.Builder, v string, indent int) {
	for i := 0; i < indent; i += 1 {
		sb.WriteRune(' ')
	}

	sb.WriteString(v)
}

func MarkdownNewline(sb *strings.Builder) {
	sb.WriteRune('\\')
	sb.WriteString("\n")
}

func _TypeToMarkdown(value reflect.Type, sb *strings.Builder, indent int, newline bool) {
	if value.Kind() == reflect.Pointer {
		_TypeToMarkdown(value.Elem(), sb, indent, newline)
	}

	if newline {
		WriteWithIndent(sb, "", indent)
	}

	/* @NOTE/TODO:
	Types like UUID or timestamp are from SQL drivers, we don't really want to dive into them.
	The problem arises if user wants his own type to be represented in a similar way.
	I don't want to overcomplicate the system yet, but we will likely need some user mapping for such situations.
	*/
	if value.Name() == "UUID" {
		sb.WriteString("uuid")
	} else if value.Name() == "Timestamp" || value.Name() == "Timestamptz" {
		sb.WriteString("timestamp")
	} else if value.Name() == "Time" {
		sb.WriteString("time")
	} else if value.Kind() == reflect.Interface && value.NumMethod() == 0 {
		sb.WriteString("any")
	} else if value.Kind() == reflect.Struct {
		sb.WriteString("{\n")

		fieldCount := value.NumField()
		for i := 0; i < fieldCount; i += 1 {
			indent := indent + 4

			handleStructField := func(field reflect.StructField) {
				jsonTag := field.Tag.Get("json")

				name := ""
				parts := strings.Split(jsonTag, ",")
				if len(parts) > 0 {
					if parts[0] != "" {
						name = parts[0]
					}
				}
				if name == "" {
					// NOTE: private fields are not exported, not because i believe in "private", but because go structs have
					// some private fields inside them that you don't want visible in documentation
					if !unicode.IsLower(rune(field.Name[0])) {
						name = field.Name
					}

				}
				if name == "" {
					return
				}

				WriteWithIndent(sb, fmt.Sprintf("%v: ", name), indent)

				_TypeToMarkdown(field.Type, sb, indent, false)
				ourTags := ParseOurTags(field)
				if ourTags.IsARequiredField {
					sb.WriteString(" (required)")
				}

				description := ParseFieldDescription(field)
				if description != "" {
					sb.WriteString(fmt.Sprintf(" // %v", description))
				}

				sb.WriteString("\n")
			}

			field := value.Field(i)

			if field.Anonymous { // @Hack: One layer deep should be enough, it is not then we can later gather all fields from anon structs
				for k := 0; k < field.Type.NumField(); k += 1 {
					field := field.Type.Field(k)

					handleStructField(field)
				}
				continue
			}

			handleStructField(field)
		}

		WriteWithIndent(sb, "}", indent)
	} else if value.Kind() == reflect.Slice {
		sb.WriteString("[\n")
		_TypeToMarkdown(value.Elem(), sb, indent+4, true)
		sb.WriteString("\n")
		WriteWithIndent(sb, "]", indent)
	} else if value.Kind() == reflect.Map {
		if value.Elem().Kind() == reflect.Interface && value.Elem().NumMethod() == 0 {
			sb.WriteString("(any data)")
		} else {
			sb.WriteString("(map)")
		}
	} else {
		sb.WriteString(value.Name())
	}
}

type OurTags struct {
	IsARequiredField bool
}

func ParseOurTags(field reflect.StructField) OurTags {
	_tags := field.Tag.Get("tag")
	tags := strings.Split(_tags, ",")
	ourTags := OurTags{}
	_, ourTags.IsARequiredField = Search(tags, func(tag string) bool {
		return tag == "required"
	})

	return ourTags
}

func ParseFieldDescription(field reflect.StructField) string {
	description := field.Tag.Get("description")
	return description
}

func GetDocumentation(context *Context, filter string) string {
	proceduresByCategory := make(map[string]*[]string, 0)
	for procedureName, procedure := range context.Procedures {
		proceduresInThisCategory, ok := proceduresByCategory[procedure.Category]
		if !ok {
			_proceduresInThisCategory := make([]string, 0)
			proceduresInThisCategory = &_proceduresInThisCategory
			proceduresByCategory[procedure.Category] = proceduresInThisCategory
		}

		*proceduresInThisCategory = append(*proceduresInThisCategory, procedureName)
	}

	type CategoryProcedures struct {
		Category   string
		Procedures *[]string
	}
	var proceduresByCategoryOrdered []CategoryProcedures
	for categoryName, procedures := range proceduresByCategory {
		proceduresByCategoryOrdered = append(proceduresByCategoryOrdered, CategoryProcedures{
			Category:   categoryName,
			Procedures: procedures,
		})
	}
	sort.SliceStable(proceduresByCategoryOrdered, func(i, j int) bool {
		return strings.Compare(proceduresByCategoryOrdered[i].Category, proceduresByCategoryOrdered[j].Category) == -1
	})
	for _, categoryProcedures := range proceduresByCategoryOrdered {
		sort.SliceStable(*categoryProcedures.Procedures, func(i, j int) bool {
			ith := (*categoryProcedures.Procedures)[i]
			jth := (*categoryProcedures.Procedures)[j]
			return strings.Compare(ith, jth) == -1
		})
	}

	var sb strings.Builder
	for _, categoryProcedures := range proceduresByCategoryOrdered {
		sb.WriteString("<details>\n")
		categoryName := categoryProcedures.Category
		if categoryName == "" {
			categoryName = "Other"
		}
		sb.WriteString(fmt.Sprintf("<summary>%v (%v)</summary>\n\n", categoryName, len(*categoryProcedures.Procedures)))

		for _, procedureName := range *categoryProcedures.Procedures {
			sb.WriteString(context.Procedures[procedureName].Documentation)
		}

		sb.WriteString("</details>\n")
	}

	return sb.String()
}
