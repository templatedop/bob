{{- define "unique_constraint_error_detection_method"}}
func (e *UniqueConstraintError) Is(target error) bool {
	{{if eq $.Driver "github.com/jackc/pgx/v5"}}
		{{/* Only pgx/v5 native driver is supported */}}
		{{$.Importer.Import "github.com/jackc/pgx/v5/pgconn"}}
		err, ok := target.(*pgconn.PgError)
		if !ok {
			return false
		}
		return err.Code == "23505" && (e.s == "" || err.ConstraintName == e.s)
	{{else}}
		{{/* Driver validation in psql.go should prevent reaching this */}}
		panic("Unsupported driver {{$.Driver}}. Only github.com/jackc/pgx/v5 is supported for PostgreSQL")
	{{end}}
}
{{end -}}
