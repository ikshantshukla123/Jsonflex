package jsonflex

import "testing"

var benchPayload = []byte(`{
  "userName": "amy",
  "emailAddress": "amy@example.com",
  "isActive": true,
  "loginCount": 42,
  "address": {
    "streetName": "5th Avenue",
    "zipCode": "10001",
    "geoLocation": {"latValue": 40.7128, "lngValue": -74.006}
  },
  "orderItems": [
    {"itemName": "pen", "unitPrice": 2, "inStock": true},
    {"itemName": "notebook", "unitPrice": 5, "inStock": false},
    {"itemName": "eraser", "unitPrice": 1, "inStock": true}
  ]
}`)

func BenchmarkToSnakeCase(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchPayload)))
	for i := 0; i < b.N; i++ {
		if _, err := ToSnakeCase(benchPayload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCamelToSnake(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		CamelToSnake("someReasonablyLongCamelCaseKey")
	}
}

func BenchmarkSnakeToCamel(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		SnakeToCamel("some_reasonably_long_snake_case_key")
	}
}
