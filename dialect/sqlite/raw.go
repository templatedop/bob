package sqlite

import (
	"github.com/templatedop/bob"
	"github.com/templatedop/bob/dialect/sqlite/dialect"
	"github.com/templatedop/bob/expr"
)

func RawQuery(q string, args ...any) bob.BaseQuery[expr.Clause] {
	return expr.RawQuery(dialect.Dialect, q, args...)
}
