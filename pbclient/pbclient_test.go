package pbclient

import (
	"fmt"
	"testing"
)

func getPbClient(t *testing.T) *Pocketbase {
	// use actual test username and password that you set up on the "installer" page
	pb, err := NewPocketbase("http://0.0.0.0:8080", "test@example.com", "1234567890", true)
	if err != nil {
		t.Fatalf("err creating new pocketbase : %v", err)
	}
	return pb
}

func TestAuthenticate(t *testing.T) {
	tests := []struct {
		identity string
		password string
		wantErr  bool
	}{
		{"test@example.com", "1234567890", false},
		{"invalidUser", "validPassword", true},
		{"validUser", "invalidPassword", true},
	}

	for _, tt := range tests {
		_, err := authenticate(ADMIN_AUTH_ENDPOINT, "http://0.0.0.0:8080", tt.identity, tt.password)
		if (err != nil) != tt.wantErr {
			t.Errorf("authenticate() with identity = %v, password = %v, error = %v, wantErr = %v", tt.identity, tt.password, err, tt.wantErr)
		}
	}
}

func TestCreate(t *testing.T) {
	pb := getPbClient(t)

	tests := []struct {
		recordData string
		wantErr    bool
	}{
		{`{
			"password":"0000000000",
			"passwordConfirm":"0000000000",
			"username": "test_username",
			"email": "test@example.com",
			"name":"test user"}`,
			false},
		// Add more test cases as needed
		{`{
			"password":"0000000000",
			"passwordConfirm":"0000000000",
			"username": "test_username",
			"email": "test@example.com",
			"name":"test user",}`, // trailing comma
			true},
	}

	for _, tt := range tests {
		out, err := pb.CreateRecord("users", tt.recordData)
		if (err != nil) != tt.wantErr {
			t.Errorf("CreateRecord() with recordData = %v, error = %v, wantErr = %v", tt.recordData, err, tt.wantErr)
		}
		if !tt.wantErr {
			fmt.Printf("CreateRecord Output : '%+v'\n", out)
		}
	}
}

func TestGetFilteredRecords(t *testing.T) {
	pb := getPbClient(t)
	out, err := pb.GetFilteredRecords("users", "")
	if err != nil {
		t.Fatalf("err getting filtered records : %v", err)
	}
	fmt.Printf("GetFilteredRecords Output : '%+v'\n", out)
}

func TestGetRecordById(t *testing.T) {
	pb := getPbClient(t)
	out, err := pb.GetRecordById("users", "93hppedbebcjwkr") // or whatever ID
	if err != nil {
		t.Fatalf("err getting record by id : %v", err)
	}
	fmt.Printf("GetRecordById Output : '%+v'\n", out)
}

func TestGetAllRecords(t *testing.T) {
	pb := getPbClient(t)
	out, err := pb.GetAllRecords("users", "", "")
	if err != nil {
		t.Fatalf("err getting record by id : %v", err)
	}
	fmt.Printf("GetAllRecords Output : '%+v'\n", out)
}
