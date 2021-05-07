package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("servegen: ")
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	serviceName := flag.String("s", "", "service name (required)")
	flag.Parse()

	if *serviceName == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := genService(*serviceName); err != nil {
		return fmt.Errorf("failed to generate service file: %w", err)
	}

	if err := genSocket(*serviceName); err != nil {
		return fmt.Errorf("failed to generate socket file: %w", err)
	}

	if err := genNginxConf(*serviceName); err != nil {
		return fmt.Errorf("failed to generate nginx config: %w", err)
	}

	return nil
}

func genService(serviceName string) error {
	f, err := os.Create(serviceName + ".service")
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, `
[Unit]
Description=description of your service

[Service]
EnvironmentFile=/opt/%s/env
ExecStart=/opt/%s/%s
Restart=always
PrivateTmp=true

[Install]
WantedBy=multi-user.target
`, serviceName, serviceName, serviceName)
	return nil
}

func genSocket(serviceName string) error {
	f, err := os.Create(serviceName + ".socket")
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, `
[Socket]
ListenStream=localhost:8080
Service=%s.service

[Install]
WantedBy=sockets.target
`, serviceName)

	return nil
}

func genNginxConf(serviceName string) error {
	f, err := os.Create(serviceName + ".conf")
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, `
log_format with_time '$remote_addr - $remote_user [$time_local] '
                     '"$request" $status $body_bytes_sent '
                     '"$http_referer" "$http_user_agent" $request_time';

upstream app {
	server localhost:8080;
}

init_by_lua_block { require "cjson" }

map $status $error {
    ~^[23]  0;
    default 1;
}

server {
	location / {
		access_log logs/access.log with_time;
		set $response_body '';
		body_filter_by_lua_block {
			local data, eof = ngx.arg[1], ngx.arg[2]
			ngx.ctx.buffered = (ngx.ctx.buffered or "") .. data
			if eof then
				ngx.var.response_body = ngx.ctx.buffered
			end
 		}
		log_by_lua_block {
			if ngx.var.error then
				local req = {
					uri=ngx.var.uri,
					headers=ngx.req.get_headers(),
					time=ngx.req.start_time(),
					method=ngx.req.get_method(),
					body=ngx.var.request_body
				}
				local resp = {
					headers=ngx.resp.get_headers(),
					status=ngx.status,
					time=ngx.now(),
					body=ngx.var.response_body
				}
				ngx.print(require "cjson".encode{request=req, response=resp})
			end
		}
		proxy_pass http://app;
	}
}
`)
	return nil
}
