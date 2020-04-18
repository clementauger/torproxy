build:
	go build -tags prod -o torproxy
	GOOS=windows GOARCH=amd64 go build -tags prod -o torproxy.exe
clean:
	rm torproxy
	rm torproxy.exe
run:
	go run .
prod:
	go run -tags prod .
