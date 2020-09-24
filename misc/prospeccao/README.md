## prospecção

**Como rodar ?**

Edite os arquivos haproxy.cfg e httpd.conf e em seguida use `docker-compose up -d`.

**Como acessar o socket diretamente ?**

Use o comando `socat`.

Exemplo: `echo "show table fe_https" | socat stdio haproxy/socket/admin.sock`

