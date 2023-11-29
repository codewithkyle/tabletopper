package helpers

import (
	"encoding/json"
	"strconv"
)

func ParseInt(s string) int {
    i, _ := strconv.Atoi(s)
    return i
}

func Marshal(v interface{}) string {
    b, _ := json.Marshal(v)
    return string(b)
}

