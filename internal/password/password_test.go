package password

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := Verify(hash, "correct horse battery staple")
	if err != nil || !ok {
		t.Fatalf("verify = %v, %v", ok, err)
	}
	ok, _ = Verify(hash, "wrong")
	if ok {
		t.Fatal("wrong password verified")
	}
}
