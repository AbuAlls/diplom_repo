package usecase

import "testing"

func TestValidateReadOnlySQL(t *testing.T) {
	ok := []string{
		"select * from documents",
		"  SELECT id, title FROM documents WHERE status = 'confirmed' LIMIT 5 ",
		"with recent as (select id from documents order by id desc limit 10) select * from recent",
		"select count(*) from plan_items;",
		"SELECT setting_value FROM nope_offset", // 'set'/'offset' must not trip the guard
	}
	for _, q := range ok {
		if err := validateReadOnlySQL(q); err != nil {
			t.Errorf("expected %q to be allowed, got %v", q, err)
		}
	}

	bad := []string{
		"",
		"   ",
		"delete from documents",
		"update documents set status = 'x'",
		"insert into documents (title) values ('x')",
		"drop table documents",
		"truncate documents",
		"alter table documents add column x int",
		"select 1; drop table documents", // statement chaining
		"select 1; select 2",             // chaining
		"create table t (id int)",
		"grant select on documents to public",
		"copy documents to '/tmp/x'",
		"explain select 1", // does not start with select/with
	}
	for _, q := range bad {
		if err := validateReadOnlySQL(q); err == nil {
			t.Errorf("expected %q to be rejected", q)
		}
	}
}
