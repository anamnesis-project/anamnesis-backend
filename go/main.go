package main

const (
	PORT = ":8080"
)

func main() {
	s := NewServer(PORT)
	s.Run()
}

