package sqlite

import (
	"github.com/templatedop/bob"
	"github.com/templatedop/bob/dialect/sqlite/dialect"
)

func Select(queryMods ...bob.Mod[*dialect.SelectQuery]) bob.BaseQuery[*dialect.SelectQuery] {
	q := &dialect.SelectQuery{}
	for _, mod := range queryMods {
		mod.Apply(q)
	}

	return bob.BaseQuery[*dialect.SelectQuery]{
		Expression: q,
		Dialect:    dialect.Dialect,
		QueryType:  bob.QueryTypeSelect,
	}
}
