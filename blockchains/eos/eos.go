package eos

import (
	"context"
	"fmt"
	"golang.org/x/sync/semaphore"
	"strings"
	"math/rand"
	"sync"
	db "../../db"
	util "../../util"
)

const userAccounts = 100


type EosCredentials struct {
	BlockProducerKeyPairs			map[string]util.KeyPair
	UserKeyPairs					map[string]util.KeyPair
	ContractKeyPairs				map[string]util.KeyPair
	BlockProducerWalletPassword		string
	MainWalletPassword				string
	WalletPasswords					map[string]string
}
/**
 * Setup the EOS test net
 * @param  int		nodes		The number of producers to make
 * @param  []Server servers		The list of relevant servers
 */
func Eos(nodes int,servers []db.Server) EosCredentials {
	blockProducers := nodes
	if(blockProducers > 22){
		blockProducers = 22;
	}
	fmt.Println("-------------Setting Up EOS-------------")
	ctx := context.TODO()
	masterIP := servers[0].Ips[0]
	masterServerIP := servers[0].Addr
	
	/**Start keosd**/
	util.SshExec(masterServerIP,fmt.Sprintf("docker exec -d whiteblock-node0 keosd --http-server-address 0.0.0.0:8900"))
	util.SshExec(masterServerIP,fmt.Sprintf("docker exec -d whiteblock-node1 keosd --http-server-address 0.0.0.0:8900"))	
	
	password := eos_createWallet(masterServerIP,0)//create the one and only wallet

	passwordNormal := eos_createWallet(masterServerIP,1)

	clientPasswords := make(map[string]string)

	fmt.Println("\n*** Get Key Pairs ***")

	

	contractAccounts := []string{
		"eosio.bpay",
		"eosio.msig",
		"eosio.names",
		"eosio.ram",
		"eosio.ramfee",
		"eosio.saving",
		"eosio.stake",
		"eosio.token",
		"eosio.vpay",
	}

	keyPairs := eos_getKeyPairs(servers)

	contractKeyPairs := eos_getContractKeyPairs(servers,contractAccounts)

	masterKeyPair := keyPairs[servers[0].Ips[0]]

	var accountNames []string
	for i := 0; i < userAccounts; i++{
		accountNames = append(accountNames,eos_getRegularName(i))
	}
	accountKeyPairs := eos_getUserAccountKeyPairs(masterServerIP,accountNames)

	eos_createGenesis(keyPairs[masterIP].PublicKey)
	eos_createConf(servers,keyPairs[masterIP])

	
	{
		nodes := 0
		sem := semaphore.NewWeighted(40)
		for _, server := range servers {
			for i,ip := range server.Ips {
				if nodes < 2 {
					nodes++
					continue
				}
				util.SshExec(server.Addr,fmt.Sprintf("docker exec -d whiteblock-node%d keosd --http-server-address 0.0.0.0:8900",i))
				clientPasswords[ip] = eos_createWallet(server.Addr, i)
				sem.Acquire(ctx,1)

				go func(serverIP string,accountKeyPairs map[string]util.KeyPair,accountNames []string,i int){
					defer sem.Release(1)
					for _,name := range  accountNames {
						util.SshExec(serverIP, fmt.Sprintf("docker exec whiteblock-node%d cleos wallet import --private-key %s", 
							i,accountKeyPairs[name].PrivateKey))
					}
					
				}(server.Addr,accountKeyPairs,accountNames,i)

			}
		}
		sem.Acquire(ctx,40)
		sem.Release(40)
	}

	{
		var wg sync.WaitGroup
		node := 0
		for _, server := range servers {
			wg.Add(1)
			go func(serverIP string, ips []string){
				defer wg.Done()
				util.Scp(serverIP, "./genesis.json", "/home/appo/genesis.json")
				util.Scp(serverIP, "./config.ini", "/home/appo/config.ini")

				for i := 0; i < len(ips); i++ {
					
					util.SshExecIgnore(serverIP, fmt.Sprintf("docker exec whiteblock-node%d mkdir /datadir/", i))
					util.SshExec(serverIP, fmt.Sprintf("docker cp /home/appo/genesis.json whiteblock-node%d:/datadir/", i))
					util.SshExec(serverIP, fmt.Sprintf("docker cp /home/appo/config.ini whiteblock-node%d:/datadir/", i))
					node++
				}
			}(server.Addr,server.Ips)
		}
		wg.Wait()
	}

	/**Step 2d**/
	{
		fmt.Println(
			util.SshExec(masterServerIP, fmt.Sprintf("docker exec whiteblock-node0 cleos wallet import --private-key %s", 
				keyPairs[masterIP].PrivateKey)))

		fmt.Println(
			util.SshExec(masterServerIP,
				fmt.Sprintf(`docker exec -d whiteblock-node0 nodeos -e -p eosio --genesis-json /datadir/genesis.json --config-dir /datadir --data-dir /datadir %s %s`,
					eos_getKeyPairFlag(keyPairs[masterIP]),
					eos_getPTPFlags(servers, 0))))
	}
	

	/**Step 3**/
	{
		var wg sync.WaitGroup
		util.SshExecIgnore(masterServerIP, fmt.Sprintf("docker exec whiteblock-node0 cleos -u http://%s:8889 wallet unlock --password %s",
			masterIP, password))
		for _, account := range contractAccounts {
			wg.Add(1)
			go func(masterServerIP string,masterIP string,account string,masterKeyPair util.KeyPair,contractKeyPair util.KeyPair){
				defer wg.Done()
				
				
				util.SshExec(masterServerIP, fmt.Sprintf("docker exec whiteblock-node0 cleos wallet import --private-key %s", 
					contractKeyPair.PrivateKey))
				
				util.SshExec(masterServerIP, fmt.Sprintf("docker exec whiteblock-node0 cleos -u http://%s:8889 create account eosio %s %s %s",
					 masterIP, account,masterKeyPair.PublicKey,contractKeyPair.PublicKey))

			}(masterServerIP,masterIP,account,masterKeyPair,contractKeyPairs[account])

		}
		wg.Wait()
	}
	/**Steps 4 and 5**/
	{
		contracts := []string{"eosio.token","eosio.msig"}
		util.SshExecIgnore(masterServerIP, fmt.Sprintf("docker exec whiteblock-node0 cleos -u http://%s:8889 wallet unlock --password %s",
				masterIP, password))
		for _, contract := range contracts {
			
			fmt.Println(util.SshExec(masterServerIP, fmt.Sprintf("docker exec whiteblock-node0 cleos -u http://%s:8889 set contract %s /opt/eosio/contracts/%s",
				masterIP, contract, contract)))
		}
	}
	/**Step 6**/

	fmt.Println(util.SshExec(masterServerIP, fmt.Sprintf("docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio.token create '[ \"eosio\", \"10000000000.0000 SYS\" ]' -p eosio.token@active",
		masterIP)))

	fmt.Println(util.SshExec(masterServerIP, fmt.Sprintf("docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio.token issue '[ \"eosio\", \"1000000000.0000 SYS\", \"memo\" ]' -p eosio@active",
		masterIP)))


	util.SshExecIgnore(masterServerIP, fmt.Sprintf("docker exec whiteblock-node0 cleos -u http://%s:8889 wallet unlock --password %s",
				masterIP, password))


	/**Step 7**/
	for i := 0 ; i < 5; i++{
		res, err := util.SshExecCheck(masterServerIP, fmt.Sprintf("docker exec whiteblock-node0 cleos -u http://%s:8889 set contract -x 1000 eosio /opt/eosio/contracts/eosio.system",
		masterIP))
		if(err == nil){
			fmt.Println("SUCCESS!!!!!")
			fmt.Println(res)
			break
		}
		fmt.Println(res)
	}
	
	
	/**Step 8**/

	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio setpriv '["eosio.msig", 1]' -p eosio@active`,
				masterIP)))

	/**Step 10a**/
	{
		sem := semaphore.NewWeighted(10)
		node := 0
		for _, server := range servers {
			for _, ip := range server.Ips {
				
				if node == 0 {
					node++
					continue
				}
				sem.Acquire(ctx,1)
				go func(masterServerIP string, masterKeyPair util.KeyPair, keyPair util.KeyPair,node int){
					defer sem.Release(1)
					fmt.Println(
						util.SshExec(masterServerIP,
							fmt.Sprintf("docker exec whiteblock-node0 cleos wallet import --private-key %s",
								keyPair.PrivateKey)))
					if node < blockProducers {
						fmt.Println(
							util.SshExec(masterServerIP,
								fmt.Sprintf(`docker exec whiteblock-node0 cleos -u http://%s:8889 system newaccount eosio --transfer %s %s %s --stake-net "1000000.0000 SYS" --stake-cpu "1000000.0000 SYS" --buy-ram "1000000 SYS"`,
									masterIP,
									eos_getProducerName(node),
									masterKeyPair.PublicKey,
									keyPair.PublicKey)))
						fmt.Println(
							util.SshExec(masterServerIP,
								fmt.Sprintf(`docker exec whiteblock-node0 cleos -u http://%s:8889 transfer eosio %s "100000.0000 SYS"`,
									masterIP,
									eos_getProducerName(node))))
					}

					
					
				}(masterServerIP,masterKeyPair,keyPairs[ip],node)
				node++
			}
		}
		sem.Acquire(ctx,10)
		sem.Release(10)
	}
	
	/**Step 11c**/
	{
		sem := semaphore.NewWeighted(10)
		node := 0
		for _, server := range servers {
			for i, ip := range server.Ips {
				
				if node == 0 {
					node++
					continue
				}
				sem.Acquire(ctx,1)

				go func(serverIP string,servers []db.Server,node int,i int,kp util.KeyPair){
					defer sem.Release(1)
					util.SshExecIgnore(serverIP, fmt.Sprintf("docker exec whiteblock-node%d mkdir -p /datadir/blocks", i))
					p2pFlags := eos_getPTPFlags(servers,node)
					if node > blockProducers {
						fmt.Println(
							util.SshExec(serverIP,
								fmt.Sprintf(`docker exec -d whiteblock-node%d nodeos --genesis-json /datadir/genesis.json --config-dir /datadir --data-dir /datadir %s %s`,
									i,
									eos_getKeyPairFlag(kp),
									p2pFlags)))
					}else{
						fmt.Println(
							util.SshExec(serverIP,
								fmt.Sprintf(`docker exec -d whiteblock-node%d nodeos --genesis-json /datadir/genesis.json --config-dir /datadir --data-dir /datadir -p %s %s %s`,
									i,
									eos_getProducerName(node),
									eos_getKeyPairFlag(kp),
									p2pFlags)))
					}
					
				}(server.Addr,servers,node,i,keyPairs[ip])
				node++
			}
		}
		sem.Acquire(ctx,10)
		sem.Release(10)
	}

	/**Step 11a**/
	{
		node := 0
		for _, server := range servers {
			for _, ip := range server.Ips {
				
				if node == 0 {
					node++
					continue
				}else if node >= blockProducers {
					break
				}

				if node % 5 == 0{
					util.SshExecIgnore(masterServerIP, fmt.Sprintf("docker exec whiteblock-node0 cleos -u http://%s:8889 wallet unlock --password %s",
						masterIP, password))
				}

				fmt.Println(
					util.SshExec(masterServerIP,
						fmt.Sprintf("docker exec whiteblock-node0 cleos --wallet-url http://%s:8900 -u http://%s:8889 system regproducer %s %s https://whiteblock.io/%s",
							masterIP,
							masterIP,
							eos_getProducerName(node),
							keyPairs[ip].PublicKey,
							keyPairs[ip].PublicKey)))
				
				node++
			}
		}
	}
	/**Step 11b**/
	fmt.Println(util.SshExec(masterServerIP,
						fmt.Sprintf("docker exec whiteblock-node0 cleos -u http://%s:8889 system listproducers",
							masterIP)))


	/**Create normal user accounts**/
	{
		sem := semaphore.NewWeighted(10)
		
		
		for _, name := range accountNames {
			sem.Acquire(ctx,1)
			go func(masterServerIP string,name string,masterKeyPair util.KeyPair,accountKeyPair util.KeyPair){
				defer sem.Release(1)
				fmt.Println(
							util.SshExec(masterServerIP,
								fmt.Sprintf("docker exec whiteblock-node1 cleos wallet import --private-key %s",
									accountKeyPair.PrivateKey)))
				fmt.Println(
					util.SshExec(masterServerIP,
						fmt.Sprintf(`docker exec whiteblock-node0 cleos -u http://%s:8889 system newaccount eosio --transfer %s %s %s --stake-net "500000.0000 SYS" --stake-cpu "2000000.0000 SYS" --buy-ram "2000000 SYS"`,
							masterIP,
							name,
							masterKeyPair.PublicKey,
							accountKeyPair.PublicKey)))

				fmt.Println(
					util.SshExec(masterServerIP,
						fmt.Sprintf(`docker exec whiteblock-node0 cleos -u http://%s:8889 transfer eosio %s "100000.0000 SYS"`,
							masterIP,
							name)))
			}(masterServerIP,name,masterKeyPair,accountKeyPairs[name])
		}
		sem.Acquire(ctx,10)
		sem.Release(10)
	}
	/**Vote in block producers**/
	{	
		node := 0
		for _, server := range servers {
			for range server.Ips {			
				node++
			}
		}
		if(node > blockProducers){
			node = blockProducers
		}
		util.SshExecIgnore(masterServerIP, fmt.Sprintf("docker exec whiteblock-node1 cleos -u http://%s:8889 wallet unlock --password %s",
				masterIP, passwordNormal))
		n := 0
		sem := semaphore.NewWeighted(10)
		for _, name := range accountNames {
			prod := 0
			if n != 0 {
				prod = rand.Intn(100) % n
			} 
		
			prod = prod % (node - 1)
			prod += 1
			sem.Acquire(ctx,1)
			go func(masterServerIP string,masterIP string,name string,prod int){
				defer sem.Release(1)
				fmt.Println(
						util.SshExec(masterServerIP,
							fmt.Sprintf("docker exec whiteblock-node1 cleos -u http://%s:8889 system voteproducer prods %s %s",
								masterIP,
								name,
								eos_getProducerName(prod))))
			}(masterServerIP,masterIP,name,prod)
			n++;
		}
		sem.Acquire(ctx,10)
		sem.Release(10)
	}
	
	/**Step 12**/
	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(
				`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio updateauth '{"account": "eosio", "permission": "owner", "parent": "", "auth": {"threshold": 1, "keys": [], "waits": [], "accounts": [{"weight": 1, "permission": {"actor": "eosio.prods", "permission": "active"}}]}}' -p eosio@owner`,
				masterIP)))

	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(
				`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio updateauth '{"account": "eosio", "permission": "active", "parent": "owner", "auth": {"threshold": 1, "keys": [], "waits": [], "accounts": [{"weight": 1, "permission": {"actor": "eosio.prods", "permission": "active"}}]}}' -p eosio@active`,
				masterIP)))

	
	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(
				`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio updateauth '{"account": "eosio.bpay", "permission": "owner", "parent": "", "auth": {"threshold": 1, "keys": [], "waits": [], "accounts": [{"weight": 1, "permission": {"actor": "eosio", "permission": "active"}}]}}' -p eosio.bpay@owner`,
				masterIP)))
	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(
				`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio updateauth '{"account": "eosio.bpay", "permission": "active", "parent": "owner", "auth": {"threshold": 1, "keys": [], "waits": [], "accounts": [{"weight": 1, "permission": {"actor": "eosio", "permission": "active"}}]}}' -p eosio.bpay@active`,
				masterIP)))
	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(
				`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio updateauth '{"account": "eosio.msig", "permission": "owner", "parent": "", "auth": {"threshold": 1, "keys": [], "waits": [], "accounts": [{"weight": 1, "permission": {"actor": "eosio", "permission": "active"}}]}}' -p eosio.msig@owner`,
				masterIP)))
	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(
				`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio updateauth '{"account": "eosio.msig", "permission": "active", "parent": "owner", "auth": {"threshold": 1, "keys": [], "waits": [], "accounts": [{"weight": 1, "permission": {"actor": "eosio", "permission": "active"}}]}}' -p eosio.msig@active`,
				masterIP)))
	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(
				`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio updateauth '{"account": "eosio.names", "permission": "owner", "parent": "", "auth": {"threshold": 1, "keys": [], "waits": [], "accounts": [{"weight": 1, "permission": {"actor": "eosio", "permission": "active"}}]}}' -p eosio.names@owner`,
				masterIP)))
	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(
				`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio updateauth '{"account": "eosio.names", "permission": "active", "parent": "owner", "auth": {"threshold": 1, "keys": [], "waits": [], "accounts": [{"weight": 1, "permission": {"actor": "eosio", "permission": "active"}}]}}' -p eosio.names@active`,
				masterIP)))
	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(
				`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio updateauth '{"account": "eosio.ram", "permission": "owner", "parent": "", "auth": {"threshold": 1, "keys": [], "waits": [], "accounts": [{"weight": 1, "permission": {"actor": "eosio", "permission": "active"}}]}}' -p eosio.ram@owner`,
				masterIP)))

	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(
				`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio updateauth '{"account": "eosio.ram", "permission": "active", "parent": "owner", "auth": {"threshold": 1, "keys": [], "waits": [], "accounts": [{"weight": 1, "permission": {"actor": "eosio", "permission": "active"}}]}}' -p eosio.ram@active`,
				masterIP)))
	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(
				`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio updateauth '{"account": "eosio.ramfee", "permission": "owner", "parent": "", "auth": {"threshold": 1, "keys": [], "waits": [], "accounts": [{"weight": 1, "permission": {"actor": "eosio", "permission": "active"}}]}}' -p eosio.ramfee@owner`,
				masterIP)))
	fmt.Println(
		util.SshExec(masterServerIP,
			fmt.Sprintf(
				`docker exec whiteblock-node0 cleos -u http://%s:8889 push action eosio updateauth '{"account": "eosio.ramfee", "permission": "active", "parent": "owner", "auth": {"threshold": 1, "keys": [], "waits": [], "accounts": [{"weight": 1, "permission": {"actor": "eosio", "permission": "active"}}]}}' -p eosio.ramfee@active`,
				masterIP)))






	util.Write("eos_info.txt",
		fmt.Sprintf("Account Key Pairs \n%+v\n\nContract Key Pairs\n%+v\n\nWallet Password\n%s\n\nWallet Password2\n%s\n\nRest of Wallets%v\n\n\n\n",
			keyPairs,contractKeyPairs,password,passwordNormal,clientPasswords))


	util.Rm("./genesis.json")
	util.Rm("./config.ini")

	return EosCredentials{
		BlockProducerKeyPairs : keyPairs,
		UserKeyPairs : accountKeyPairs,
		ContractKeyPairs : contractKeyPairs,
		BlockProducerWalletPassword : password,
		MainWalletPassword : passwordNormal,
		WalletPasswords : clientPasswords,
	}
	
}
func eos_getKeyPair(serverIP string) util.KeyPair {
	data := util.SshExec(serverIP, "docker exec whiteblock-node0 cleos create key --to-console | awk '{print $3}'")
	//fmt.Printf("RAW KEY DATA%s\n", data)
	keyPair := strings.Split(data, "\n")
	if(len(data) < 10){
		fmt.Printf("Unexpected create key output %s\n",keyPair)
		panic(1)
	}
	return util.KeyPair{PrivateKey: keyPair[0], PublicKey: keyPair[1]}
}


func eos_getKeyPairs(servers []db.Server) map[string]util.KeyPair {
	keyPairs := make(map[string]util.KeyPair)
	/**Get the key pairs for each nodeos account**/
	
	var wg sync.WaitGroup
	var mutex = &sync.Mutex{}

	for _, server := range servers {
		wg.Add(1)
		go func(serverIP string,ips []string){
			defer wg.Done()
			for _, ip := range ips {
				data := util.SshExec(serverIP, "docker exec whiteblock-node0 cleos create key --to-console | awk '{print $3}'")
				//fmt.Printf("RAW KEY DATA%s\n", data)
				keyPair := strings.Split(data, "\n")
				if(len(data) < 10){
					fmt.Printf("Unexpected create key output %s\n",keyPair)
					panic(1)
				}
					
				mutex.Lock()
				keyPairs[ip] = util.KeyPair{PrivateKey: keyPair[0], PublicKey: keyPair[1]}
				mutex.Unlock()
			}
		}(server.Addr,server.Ips)
	}
	wg.Wait()
	
	return keyPairs
}


func eos_getContractKeyPairs(servers []db.Server,contractAccounts []string) map[string]util.KeyPair {

	keyPairs := make(map[string]util.KeyPair)
	server := servers[0]

	for _,contractAccount := range contractAccounts {
		
		keyPairs[contractAccount] = eos_getKeyPair(server.Addr)
	}
	return keyPairs
}

func eos_getUserAccountKeyPairs(masterServerIP string,accountNames []string) map[string]util.KeyPair {

	keyPairs := make(map[string]util.KeyPair)
	sem := semaphore.NewWeighted(10)
	var mutex = &sync.Mutex{}
	ctx := context.TODO()

	for _,name := range accountNames {
		sem.Acquire(ctx,1)
		go func(serverIP string,name string){
			data := util.SshExec(serverIP, "docker exec whiteblock-node0 cleos create key --to-console | awk '{print $3}'")
			//fmt.Printf("RAW KEY DATA%s\n", data)
			keyPair := strings.Split(data, "\n")
			if(len(data) < 10){
				fmt.Printf("Unexpected create key output %s\n",keyPair)
				panic(1)
			}
			mutex.Lock()
			keyPairs[name] = util.KeyPair{PrivateKey: keyPair[0], PublicKey: keyPair[1]}
			mutex.Unlock()
			sem.Release(1)
		}(masterServerIP,name)
	}
	sem.Acquire(ctx,10)
	sem.Release(10)
	return keyPairs
}


func eos_createWallet(serverIP string, id int) string {
	data := util.SshExec(serverIP, fmt.Sprintf("docker exec whiteblock-node%d cleos wallet create --to-console | tail -n 1",id))
	//fmt.Printf("CREATE WALLET DATA %s\n",data)
	offset := 0
	for data[len(data) - (offset + 1)] != '"' {
		offset++
	}
	offset++
	data = data[1 : len(data) - offset]
	fmt.Printf("CREATE WALLET DATA %s\n",data)
	return data
}


func eos_createGenesis(masterPublicKey string) {
	genesisData := fmt.Sprintf (
`{
	"initial_timestamp": "2018-10-07T12:11:00.000",
	"initial_key": "%s",
	"initial_configuration": {
		"max_block_net_usage": 1048576,
		"target_block_net_usage_pct": 1000,
		"max_transaction_net_usage": 524288,
		"base_per_transaction_net_usage": 12,
		"net_usage_leeway": 500,
		"context_free_discount_net_usage_num": 20,
		"context_free_discount_net_usage_den": 100,
		"max_block_cpu_usage": 100000,
		"target_block_cpu_usage_pct": 500,
		"max_transaction_cpu_usage": 50000,
		"min_transaction_cpu_usage": 100,
		"max_transaction_lifetime": 3600,
		"deferred_trx_expiration_window": 600,
		"max_transaction_delay": 3888000,
		"max_inline_action_size": 4096,
		"max_inline_action_depth": 4,
		"max_authority_depth": 6
	},
	"initial_chain_id": "6469636b627574740a"
}`, masterPublicKey)

	util.Write("genesis.json", genesisData)
}

func eos_getKeyPairFlag(keyPair util.KeyPair) string {
	return fmt.Sprintf("--private-key '[ \"%s\",\"%s\" ]'", keyPair.PublicKey, keyPair.PrivateKey)
}


/**
 * Create the Config file for the EOS Nodes
 * @param  []Server servers The list of servers
 */
func eos_createConf(servers []db.Server, keyPair util.KeyPair) {
	constantEntries := []string{
		"bnet-endpoint = 0.0.0.0:4321",
		"bnet-no-trx = false",
		"blocks-dir = /datadir/blocks/",
		"chain-state-db-size-mb = 8192",
		"reversible-blocks-db-size-mb = 340",
		"contracts-console = false",
		"https-client-validate-peers = 0",
		"access-control-allow-credentials = false",
		"p2p-max-nodes-per-host = 4",
		"allowed-connection = any",
		"max-clients = 0",//no limit
		"connection-cleanup-period = 30",
		"network-version-match = 0",
		"sync-fetch-span = 100",
		"max-implicit-request = 1500",
		/*"enable-stale-production = true",*/
		"pause-on-startup = false",
		"max-transaction-time = 30",
		"max-irreversible-block-age = -1",
		"keosd-provider-timeout = 5",
		"txn-reference-block-lag = 0",
		"http-server-address = 0.0.0.0:8889",
		"p2p-listen-endpoint = 0.0.0.0:8999",
		/*"agent-name = \"EOS Test Agent\"",*/ 
		"plugin = eosio::chain_plugin",
		"plugin = eosio::chain_api_plugin",
		"plugin = eosio::producer_plugin",
		"plugin = eosio::http_plugin",
		"plugin = eosio::history_api_plugin",
		"plugin = eosio::net_plugin",
		"plugin = eosio::net_api_plugin",
	}
	confData := util.CombineConfig(constantEntries)
	/*for _, server := range servers {
		for _, ip := range server.ips {
			confData += fmt.Sprintf("p2p-peer-address = %s:8999\n", ip)
		}
	}*/
	util.Write("config.ini", confData)
}


func eos_getProducerName(num int) string {
	switch(num){
		case 0:
			return "eosio"
	}
	if num == 0 {
		
	}
	out := ""

	for i := num; i > 0; i = (i - (i % 4)) / 4{
		place := i % 4
		place++
		out = fmt.Sprintf("%d%s",place,out)//I hate this
	}
	for i := len(out); i < 5; i++ {
		out = "x"+out
	}

	return "prod"+out
}

func eos_getRegularName(num int) string {

	out := ""
	//num -= blockProducers

	for i := num; i > 0; i = (i - (i % 4)) / 4{
		place := i % 4
		place++
		out = fmt.Sprintf("%d%s",place,out)//I hate this
	}
	for i := len(out); i < 8; i++ {
		out = "x"+out
	}

	return "user"+out
}


func eos_getPTPFlags(servers []db.Server, exclude int) string {
	flags := ""
	node := 0
	for _, server := range servers {
		for _, ip := range server.Ips {
			if(node == exclude){
				node++
				continue
			}
			flags += fmt.Sprintf("--p2p-peer-address %s:8999 ", ip)

		}
	}
	return flags
}