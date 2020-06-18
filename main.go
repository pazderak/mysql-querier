package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type viaSSHDialer struct {
	client *ssh.Client
}

func (vsd *viaSSHDialer) Dial(addr string) (net.Conn, error) {
	return vsd.client.Dial("tcp", addr)
}

func main() {
	sshHost := flag.String("ssh-host", "", "defines SSH jump host name for MySQL connection") // SSH Server Hostname/IP
	sshPort := flag.Int("ssh-port", 22, "defines SSH jump host port for MySQL connection")    // SSH Port
	sshUser := flag.String("ssh-user", "", "defines SSH username for jump host")              // SSH Username
	sshPass := flag.String("ssh-password", "", "defines password for jump host")              // Empty string for no password
	dbUser := flag.String("db-user", "", "DB user name")                                      // DB username
	dbPass := flag.String("db-password", "", "DB password")                                   // DB Password
	dbHost := flag.String("db-host", "127.0.0.1:3306", "DB host name (including port)")       // DB Hostname/IP
	dbName := flag.String("db-name", "", "DB name")                                           // Database name
	dbQuery := flag.String("db-query", "", "DB query to run")

	flag.Parse()

	var agentClient agent.Agent
	// Establish a connection to the local ssh-agent
	if conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		defer conn.Close()

		// Create a new instance of the ssh agent
		agentClient = agent.NewClient(conn)
	}

	// The client configuration with configuration option to use the ssh-agent
	sshConfig := &ssh.ClientConfig{
		User: *sshUser,
		Auth: []ssh.AuthMethod{},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// When the agentClient connection succeeded, add them as AuthMethod
	if agentClient != nil {
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeysCallback(agentClient.Signers))
	}
	// When there's a non empty password add the password AuthMethod
	if *sshPass != "" {
		sshConfig.Auth = append(sshConfig.Auth, ssh.PasswordCallback(func() (string, error) {
			return *sshPass, nil
		}))
	}

	// Connect to the SSH Server
	sshsrv := fmt.Sprintf("%s:%d", *sshHost, *sshPort)
	sshcon, err := ssh.Dial("tcp", sshsrv, sshConfig)
	if err != nil {
		log.Fatalf("error when connecting SSH server %s: %v\n", sshsrv, err)
	}
	defer sshcon.Close()

	// Now we register the ViaSSHDialer with the ssh connection as a parameter
	mysql.RegisterDial("mysql+tcp", (&viaSSHDialer{sshcon}).Dial)

	// And now we can use our new driver with the regular mysql connection string tunneled through the SSH connection
	dbconn := fmt.Sprintf("%s:%s@mysql+tcp(%s)/%s", *dbUser, *dbPass, *dbHost, *dbName)
	db, err := sql.Open("mysql", dbconn)
	if err != nil {
		log.Fatalf("error when connecting to DB server '%s': %v\n", *dbHost, err)
	}

	fmt.Printf("Successfully connected to the db %s\n", *dbHost)

	rows, err := db.Query(*dbQuery)
	if err != nil {
		log.Fatalf("error when running query\n'%s'\n%v\n", *dbQuery, err)
	}
	cols, err := rows.Columns()
	if err != nil {
		log.Fatalf("error when getting list of columns: %v\n", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug|tabwriter.TabIndent)
	h := strings.Join(cols, "\t")
	fmt.Fprintln(w, h)
	w.Flush()
	fmt.Fprintln(os.Stdout, strings.Repeat("-", len(h)))

	vals := make([]vscanner, len(cols))
	pointers := make([]interface{}, len(cols))
	args := make([]interface{}, len(cols))
	valf := ""
	for i := 0; i < len(vals); i++ {
		vals[i] = vscanner("")
		pointers[i] = &vals[i]
		valf += "%v"
		if i < len(vals)-1 {
			valf += "\t"
			continue
		}
		valf += "\n"
	}

	for rows.Next() {
		rows.Scan(pointers...)
		for i := range pointers {
			args[i] = *pointers[i].(*vscanner)
		}
		fmt.Fprintf(w, valf, args...)
	}
	w.Flush()
	if err := rows.Close(); err != nil {
		log.Fatalf("error when closing dataset: %v\n", err)
	}
	if err := db.Close(); err != nil {
		log.Fatalf("error when closing database connection: %v\n", err)
	}
}

type vscanner string

func (v *vscanner) Scan(src interface{}) error {
	var source string
	switch src.(type) {
	case string:
		source = src.(string)
	case []byte:
		source = string(src.([]byte))
	default:
		return fmt.Errorf("unknown type %T", src)
	}
	*v = vscanner(source)
	return nil
}
