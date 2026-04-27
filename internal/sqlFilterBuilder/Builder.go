package sqlfilterbuilder

import (
	"strings"
)

type Builder struct {
	hasFilters bool
	qb         strings.Builder
	args       []any
}

func New() *Builder {
	return &Builder{}
}

func (fj *Builder) AddEqFilter(columnName string, value string) {
	if value == "" {
		return
	}
	fj.checkHasFilter()
	fj.qb.WriteString("`" + columnName + "` = ?\n")
	fj.args = append(fj.args, value)
}

func (fj *Builder) AddLikeFilter(columnName string, value string) {
	if value == "" {
		return
	}
	fj.checkHasFilter()
	fj.qb.WriteString("lower(`" + columnName + "`) LIKE lower(?)\n")
	fj.args = append(fj.args, "%"+value+"%")
}

func (fj *Builder) AddInFilter(columnName string, values []int) {
	if len(values) == 0 {
		return
	}
	fj.checkHasFilter()
	// As we're on sqlite, we don't need any Rebind or anything like that.
	placeholders := strings.Repeat("?,", len(values))
	fj.qb.WriteString("`" + columnName + "` IN (" + placeholders[:len(placeholders)-1] + ")\n")
	for _, v := range values {
		fj.args = append(fj.args, v)
	}
}

func (fj *Builder) AddFilter(filter string) {
	fj.checkHasFilter()
	fj.qb.WriteString(filter)
}

func (fj *Builder) AddParamFilter(filter string, args ...any) {
	fj.checkHasFilter()
	fj.qb.WriteString(filter)
	fj.args = append(fj.args, args...)
}

func (fj *Builder) String() string {
	return fj.qb.String()
}

func (fj *Builder) Args() []any {
	return fj.args
}

func (fj *Builder) HasFilters() bool {
	return fj.hasFilters
}

func (fj *Builder) checkHasFilter() {
	if !fj.hasFilters {
		fj.hasFilters = true
	} else {
		fj.qb.WriteString(" AND ")
	}
}
