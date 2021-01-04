all: Factotum

linux: main.go
	go build -o cmd/factotum main.go

windows: main.go
	GOOS=windows GOARCH=386 go build -o cmd/factotum.exe main.go

test: godeploy
	./godeploy sth
