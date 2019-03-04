clean:
	rm -rf dist

build:
	GOOS=linux GOARCH=amd64 go build -o dist/stargazer .
	cp ./favicon.ico dist/
	cp -R ./templates dist/

package:
	(cd dist && tar -czvf ../dist.tar.gz .)
	docker build -t stargazer:build .