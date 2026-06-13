package payment

import "testing"

func TestVerifyHMAC(t *testing.T) {
	secret := "s3cret"
	body := []byte(`{"order_id":"abc","status":"paid"}`)
	sig := SignHMAC(secret, body)

	if !VerifyHMAC(secret, body, sig) {
		t.Fatal("valid signature rejected")
	}
	if VerifyHMAC("wrong", body, sig) {
		t.Fatal("wrong secret accepted")
	}
	if VerifyHMAC(secret, []byte(`{"order_id":"abc","status":"refunded"}`), sig) {
		t.Fatal("tampered body accepted")
	}
	if VerifyHMAC(secret, body, "not-hex") {
		t.Fatal("malformed signature accepted")
	}
}
