package main

type Route struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Endpoint string `json:"endpoint"`
	Local    bool   `json:"local"`
}
