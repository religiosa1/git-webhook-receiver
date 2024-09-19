package actiondb

import "strings"

type filterJoiner struct {
	HasFilters bool
	qb         strings.Builder
	args       []interface{}
}

func (fj *filterJoiner) AddLikeFilter(value string, columnName string) {
	if value == "" {
		return
	}
	fj.checkHasFilter()
	fj.qb.WriteString("`" + columnName + "` LIKE ?\n")
	fj.args = append(fj.args, "%"+value+"%")
}

func (fj *filterJoiner) AddFilter(filter string) {
	fj.checkHasFilter()
	fj.qb.WriteString(filter)
}

func (fj *filterJoiner) checkHasFilter() {
	if !fj.HasFilters {
		fj.HasFilters = true
	} else {
		fj.qb.WriteString(" AND ")
	}
}

func (fj *filterJoiner) String() string {
	return fj.qb.String()
}

func (fj *filterJoiner) Args() []interface{} {
	return fj.args
}
