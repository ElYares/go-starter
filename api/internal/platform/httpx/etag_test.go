package httpx

import "testing"

// Lo que ETag escribe tiene que ser lo que VersionFromIfMatch lee. Si uno
// cambia las comillas y el otro no, toda modificacion responde 400.
func TestElETagQueSeEscribeEsElQueSeLee(t *testing.T) {
	for _, v := range []int{1, 7, 1234} {
		got, prob := VersionFromIfMatch(ETag(v))
		if prob != nil || got != v {
			t.Errorf("ETag(%d) = %s, leido = %d, problema = %v", v, ETag(v), got, prob)
		}
	}
}

func TestIfMatchAceptaLaFormaDebilYSinComillas(t *testing.T) {
	for _, crudo := range []string{`W/"7"`, `7`, ` "7" `} {
		if got, prob := VersionFromIfMatch(crudo); prob != nil || got != 7 {
			t.Errorf("%q: version = %d, problema = %v", crudo, got, prob)
		}
	}
}

// `*` es una sobrescritura incondicional: justo lo que If-Match existe para
// impedir. Y una version que no es un contador positivo no es de este recurso.
func TestIfMatchRechazaElComodinYLoQueNoEsUnaVersion(t *testing.T) {
	for _, crudo := range []string{`*`, `"abc"`, `"0"`, `"-3"`, ``} {
		_, prob := VersionFromIfMatch(crudo)
		if prob == nil {
			t.Errorf("%q se acepto", crudo)
			continue
		}
		if prob.Status != 400 || len(prob.Errors) != 1 || prob.Errors[0].Field != "If-Match" {
			t.Errorf("%q: problema = %+v", crudo, prob)
		}
	}
}
