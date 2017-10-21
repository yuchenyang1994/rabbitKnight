package rabbit

import (
	"fmt"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	test := NewKnightConfigManager("../config/test.yml")
	allQueue := test.LoadQueuesConfig()
	fmt.Println(allQueue)

}
