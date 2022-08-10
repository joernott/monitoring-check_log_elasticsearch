package check

import (
	"fmt"
	"sort"
	"strings"
)

// Basically a string in the format "<order>Þ<rulename>". This is used for the Get function
type OrderedRuleKey string

// This is used to order the rules
type OrderedRuleList []OrderedRuleKey

// Append a rule with its order to the list
func (o OrderedRuleList) Append(RuleName string, Order int) OrderedRuleList {
	s := fmt.Sprintf("%06dÞ%v", Order, RuleName)
	return append(o, OrderedRuleKey(s))
}

// Sort the list
func (o OrderedRuleList) Sort() OrderedRuleList {
	sort.Sort(o)
	return o
}

// Get the Rule name and rule using the key and the list of rules
func (k OrderedRuleKey) Get(List RuleList) (string, Rule) {
	n := strings.Split(string(k), "Þ")
	rulename := strings.Join(n[1:], "")
	return rulename, List[rulename]
}

// Used for sorting
func (o OrderedRuleList) Len() int {
	return len(o)
}

// Used for sorting
func (o OrderedRuleList) Less(i, j int) bool {
	return o[i] < o[j]
}

// Used for sorting
func (o OrderedRuleList) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}
