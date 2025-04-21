
# Retesteth - bor

These integration tests are included in the bor repo via using the git submodule  

```
[submodule "tests/testdata"]
	path = tests/testdata
	url = https://github.com/zenanetwork/tests.git
```

The version used is the last stable release, tagged as v10.4 from branch develop in zenanet/tests    
Details on release code can be found here https://github.com/zenanetwork/tests/commit/a380655e5ffab1a5ea0f4d860224bdb19013f06a  

To run the tests, we have a `make` command:  
``` 
make test-integration
```
which is also integrated into the CI pipeline on GitHub  


## Retesteth - bor on remote machine

To explore and test the `retesteth` package, the following steps were executed.  
This is only for educational purposes.  
For future usage, there is no need to go through this section, the only thing needed is to have 'green' integration tests.  

- `ssh` into a VM running bor 
- Change configs by replacing gzen with bor inside the docker container  
```
mkdir ~/retestethBuild
cd ~/retestethBuild
wget https://raw.githubusercontent.com/zenanet/retesteth/develop/dretesteth.sh
chmod +x dretesteth.sh
wget https://raw.githubusercontent.com/zenanet/retesteth/develop/Dockerfile
```

Modify the RUN git clone line in the Dockerfile for repo “retesteth” to change branch -b from master to develop. Do not modify repo branches for “winsvega/solidity” [LLLC opcode support] and “go-zenanet”.
Modify the Dockerfile so that the eth client points to bor  
e.g. : `https://github.com/zenanetwork/retesteth/blob/master/Dockerfile#L41`
from `RUN git clone --depth 1 -b master https://github.com/zenanetwork/go-zenanet.git /gzen`
to: `RUN git clone --depth 1 -b master https://github.com/maticnetwork/bor.git /gzen`

- build docker image
`sudo ./dretesteth.sh build`

- clone repo
``` 
git clone --branch develop https://github.com/zenanetwork/tests.git
```
this step is eventually replaced by adding the git submodule directly into bor repo with   
``` 
git submodule add --depth 1 https://github.com/zenanetwork/tests.git tests/testdata
```
- Let's move to the restestethBuild folder
```
cd /home/ubuntu/retestethBuild
```
Now we have the tests repo here  
```
ls
> Dockerfile  dretesteth.sh  tests
```
- Run test example    
```
./dretesteth.sh -t GeneralStateTests/stExample --  --testpath /home/ubuntu/retestethBuild/tests --datadir /tests/config
```
This will create the config files for the different clients in `~/tests/config`
Eventually, these configuration files need to be adapted according to the following document:

https://zenanet-tests.readthedocs.io/en/latest/retesteth-tutorial.html

Specifically, if you look inside `~/tests/config`, you'll see a directory for each configured client. Typically this directory contains the following:

* `config`: Contains the test configuration for the client
    * The communication protocol to use with the client (typically TCP).
    * The address(es) to use with that protocol.
    * The forks supported by the client.
    * The exceptions the client can throw, and how retesteth should interpret them. This is particularly important when testing the client's behavior when given invalid blocks.
* `start.sh`: Starts the client inside the Docker image
* `stop.sh`: Stops the client instance(s)
* `genesis`: A directory which includes the genesis blocks for the various forks supported by the cient. If this directory does not exist for a client, it uses the genesis blocks for the default client.

We replaced gzen inside docker by using https://zenanet-tests.readthedocs.io/en/latest/retesteth-tutorial.html#replace-gzen-inside-the-docker  
Theoretically, we would not need any additional config change  

- Run test suites    
``` 
./dretesteth.sh -t <TestSuiteName> --  --testpath /home/ubuntu/retestethBuild/tests --datadir /tests/config
```
Where `TestSuiteName` is one of the maintained test suites, reported here https://github.com/zenanetwork/tests  
```
BasicTests
BlockchainTests
GeneralStateTests
TransactionTests
RLPTest
src
```

If you want to run retestheth against a bor client on localhost:8545 (using 8 threads), instead of isolating it into a docker image, run  
`sudo ./dretesteth.sh -t GeneralStateTests -- --testpath ~/tests --datadir /tests/config --clients t8ntool --nodes 127.0.0.1:8545 -j 8`
