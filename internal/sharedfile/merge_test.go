package sharedfile

import (
	"reflect"
	"testing"
)

func TestMergeWholeEntriesAndOrder(t *testing.T) {
	type item struct{ Name, Value string }
	shared := []item{{"a", "shared"}, {"b", "shared"}, {"c", "shared"}}
	local := []item{{"b", "local"}, {"d", "local"}}
	for _, tc := range []struct {
		skip []string
		want []item
	}{
		{nil, []item{{"a", "shared"}, {"c", "shared"}, {"b", "local"}, {"d", "local"}}},
		{[]string{"a"}, []item{{"c", "shared"}, {"b", "local"}, {"d", "local"}}},
		{[]string{"[ac]"}, local}, {[]string{"*"}, local},
	} {
		got, origins := Merge(shared, local, func(x item) string { return x.Name }, tc.skip)
		if !reflect.DeepEqual(got, tc.want) || origins["b"] != OverridesShared || origins["d"] != Local {
			t.Fatalf("merge=%v origins=%v", got, origins)
		}
	}
	if ValidSkipPattern("[") == nil || ValidSkipPattern("*") != nil {
		t.Fatal("glob validation")
	}
}
