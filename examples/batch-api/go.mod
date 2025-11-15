module github.com/stephenafamo/bob/examples/batch-api

go 1.23

require (
	github.com/go-chi/chi/v5 v5.2.0
	github.com/jackc/pgx/v5 v5.7.5
	github.com/stephenafamo/bob v0.41.1
)

replace github.com/stephenafamo/bob => ../..
