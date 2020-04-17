build:
	go build -tags prod -o torproxy
	GOOS=windows GOARCH=amd64 go build -tags prod -o torproxy.exe
clean:
	rm bindata.go
	rm torproxy
run:
	go run .
prod:
	go run -tags prod .
