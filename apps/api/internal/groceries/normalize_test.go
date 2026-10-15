package groceries

import "testing"

func TestNormalizeRawText(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Leche  ent. HACENDADO 1l": "LECHE ENT. HACENDADO 1L",
		"  Jamón\tserrano  ":       "JAMON SERRANO",
		"Piña  COLADA\n":           "PINA COLADA",
		"":                         "",
		"   ":                      "",
		"YA NORMALIZADO":           "YA NORMALIZADO",
	}
	for in, want := range cases {
		if got := NormalizeRawText(in); got != want {
			t.Errorf("NormalizeRawText(%q) = %q, want %q", in, got, want)
		}
	}
}
