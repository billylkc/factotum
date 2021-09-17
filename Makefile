all: Factotum

linux: main.go
	go build -o bin/factotum main.go

windows: main.go
	GOOS=windows GOARCH=386 go build -o bin/factotum.exe main.go

test: godeploy
	./godeploy sth
