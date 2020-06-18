# MySQL querier

MySQL querier is able to run SQL script defined by file on MySQL server. It can connect directly or via SSH tunnel.

## Usage

```go
./mysql-querier --ssh-host <ssh-host> --ssh-user <ssh-user> (--ssh-password <ssh-password>) --db-host <db-host> --db-user <db-user> --db-password <db-password> (--db-name <db-name>) --db-query "<db-query>"
```
