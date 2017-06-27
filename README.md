# possum

[![codecov.io](https://codecov.io/github/FidelityInternational/possum/coverage.svg?branch=master)](https://codecov.io/github/FidelityInternational/possum?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/FidelityInternational/possum)](https://goreportcard.com/report/github.com/FidelityInternational/possum)
[![Build Status](https://travis-ci.org/FidelityInternational/possum.svg?branch=master)](https://travis-ci.org/FidelityInternational/possum)

Possum is an application that can be deployed to Cloud Foundry that is intended to be used as a Health Monitor endpoint for a loadbalancer. It allows you to fake the death of a Cloud Foundry Foundation to trigger load balancer traffic routing changes in multi-foundation CF environments.

IMPORTANT: Possum supports setting of the Cross-Origin-Resource-Scripting (CORS) Headers to allow calling of possum URLs from inside Javascript on other sites. The default value for this header is '*' which means that it will allow calling from anywhere. This may not be in line with your desired security config, so be sure to set a sensible `CORS_ALLOWED` environment variable when deploying.

![Cross-site Possum](heidi.jpg "Cross-site Possum")

### Prereqs

* Optional: If you are using possum on `https` endpoints with self-signed/ private certificates you will need to add a `cacert.pem` file to the root of this repository before deploy.

### Deploy

What the script does:
* Creates an Org and Space to deploy possum to
* Sets up required user-provided-services - these are configuration only!
* Deploys the possum application
* Zero downtime upgrades possum if it is already deployed

What the script does **NOT** do:
* Create a database - possum requires a SQL database, the deploy script will configure the application to connect to an extant database.

**If you want to use a managed service to provide your database** like p-mysql, or clear db you can!
* Create your service with `cf create-service p-mysql 512mb possum-db`
  * Ensure that the name of the service is *possum-db*
* Omit the `DB_` variables mentioned in the table below
* Run `deploy.sh` with the `--managed-db` argument.

Required Variables:

| Variable             | Required | Description                                                                                                                                                                                                            |
|----------------------|----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| CF_DEPLOY_USERNAME   | Yes      | A Cloud Foundry user with rights to create orgs/ spaces and deploy the applications                                                                                                                                    |
| CF_DELOY_PASSWORD    | Yes      | The password for CF_DEPLOY_USERNAME                                                                                                                                                                                    |
| CF_SYS_DOMAIN        | Yes      | The system domain of Cloud Foundry, the deployment script will need to connect to https://api.CF_SYSTEM_DOMAIN and possum will need to connect to both https://api.CF_SYSTEM_DOMAIN and https://login.CF_SYSTEM_DOMAIN |
| PASSEL               | Yes      | A JSON array of URIs of all Possums in your Passel (Cluster of possums)                                                                                                                                                |
| POSSUM_USERNAME      | Yes      | The username that you would like to use to secure the POST endpoint of possums, this is basic auth and needs to be set the same for all possums in the Passel                                                          |
| POSSUM_PASSWORD      | Yes      | The password that you would like to use to secure the POST endpoint of possums, this is basic auth and needs to be set the same for all possums in the Passel                                                          |
| POSSUM_ORG           | Optional | The CF Org to create and deploy into. The default is possum                                                                                                                                                            |
| POSSUM_SPACE         | Optional | The CF Space to create and deploy into. The default is possum                                                                                                                                                          |
| POSSUM_APP_NAME      | Optional | The name to deploy the app under (i.e. possum-test). The default is possum                                                                                                                                             |
| DB_NAME              | Optional | The database that possum will use. Required for `cups` provided database.                                                                                                                                              |
| DB_HOST              | Optional | The IP address or hostname of the database. Required for `cups` provided database.                                                                                                                                     |
| DB_PORT              | Optional | The port used to connect to the database. Required for `cups` provided database.                                                                                                                                       |
| DB_USERNAME          | Optional | A database user with rights to create and update tables. Required for `cups` provided database.                                                                                                                        |
| DB_PASSWORD          | Optional | The password for DB_USERNAME. Required for `cups` provided database.                                                                                                                                                   |
| GLOBAL_DOMAIN        | Optional | If you have a single domain that can route to multiple CF foundations you may want to map the same URI to multiple instances in a possum cluster. Note: This domain should never be added to the PASSEL variable above |
| CORS_ALLOWED         | Optional | Defaults to '*' if no more specific match supplied as ENV variable                 |


Example deploy with user provided database:

```
CF_SYS_DOMAIN='system_domain.example.com' \
CF_DEPLOY_USERNAME='an_admin_user' \
CF_DEPLOY_PASSWORD='an_admin_user_password' \
DB_NAME='database_name' \
DB_HOST='database_hostname_or_ip' \
DB_PORT='database_port' \
DB_USERNAME='a_database_user' \
DB_PASSWORD='a_database_user_password' \
PASSEL=<'["https://possum.apps.cf-foundation1.com", "https://possum2.apps.cf-foundation1.com"]'> \
POSSUM_USERNAME=<username> \
POSSUM_PASSWORD=<password> \
POSSUM_ORG=<possum-test> \
POSSUM_SPACE=<possum-test1> \
POSSUM_APP_NAME=<possum-test1> \
CORS_ALLOWED=<> \
deploy.sh
```

Example deploy with managed database

```
CF_SYS_DOMAIN=<sysdomain.example.com> \
CF_DEPLOY_USERNAME=<cf_admin_username> \
CF_DEPLOY_PASSWORD=<cf_admin_password> \
PASSEL=<'["https://possum.apps.cf-foundation1.com", "https://possum2.apps.cf-foundation1.com"]'> \
POSSUM_USERNAME=<username> \
POSSUM_PASSWORD=<password> \
POSSUM_ORG=<possum-test> \
POSSUM_SPACE=<possum-test1> \
POSSUM_APP_NAME=<possum-test1> \
CORS_ALLOWED=<> \
./deploy.sh --managed-db
```

### Usage

| Endpoint                     | Method | Description                                                                                                           | Options                                            |
|------------------------------|--------|-----------------------------------------------------------------------------------------------------------------------|----------------------------------------------------|
| /v1/state                    | GET    | Returns the state for the current possum as long as it is part of the configured Passel                               |                                                    |
| /v1/passel_state             | GET    | Returns the states for all possums in the configured Passel                                                           |                                                    |
| /v1/passel_state_consistency | GET    | Returns the states for all possums in a given passel and checks that all possums have a consistent view of the passel |                                                    |
| /v1/state                    | POST   | Configures the state of the passel for a single possum (as each possum has its own db)                                |                                                    |
| /v1/passel_state             | POST   | Configures the state of the passel for all possums in the passel, ensuring consistency                                | force - dont check state consistency before update |


#### Examples

##### GET /v1/passel_state

```
curl -k https://possum.apps.cf-foundation1.com/v1/passel_state
```

##### POST /v1/state

```
curl -kX POST https://username:password@possum.apps.cf-foundation1.com/v1/state -d '{"https://possum.apps.cf-foundation1.com": "alive"}'
```

##### POST /v1/passel_state

Set passel state without force:

```
curl -kX POST https://username:password@possum.apps.cf-foundation1.com/v1/passel_state -d '{"possum_states":{"https://possum.apps.cf-foundation1.com": "alive", "https://possum.apps.cf-foundation2.com": "alive"}}'
```

Set passel state with force:

```
curl -kX POST https://username:password@possum.apps.cf-foundation1.com/v1/passel_state -d '{"possum_states":{"https://possum.apps.cf-foundation1.com": "alive", "https://possum.apps.cf-foundation2.com": "alive"}, "force": true}'
```


### Smoke Tests

This will perform non-disruptive smoke tests against the provided APP_URL by issuing some GET requests and confirming the results look correct.

#### Example

```
APP_URL=https://possum.apps.cf-foundation1.com ./smoke-tests.sh
```
