
# Wall-Eve

  

Wall-Eve is an API that provide aggregated data from the market of the game Eve-Online.

  

Eve-Online is a MMORPG where trading is an important aspect of the game. The developers provide a lot of endpoints to help the development of third parties tool but the endpoint for the market does not provide aggregated data, meaning that most of the developers creating market application have to pull the data, aggregate it and then work with it.

  

Wall-Eve is a possible solution to this problem. It aggregate the data and let people retrieve informations using the API exposed by this application. They even can filter using some query parameters.

The application will refresh the data regurarly to provide up-to-data informations.

  
  

![A schema of the application](dist/archi.png.jpg?raw=true "Application")

  

# Overview video (Optional)

  

Here's a short video that explains the project and how it uses Redis:

  

[Insert your own video here, and remove the one below]

  

[![Embed your YouTube video](https://i.ytimg.com/vi/vyxdC1qK4NE/maxresdefault.jpg)](https://www.youtube.com/watch?v=vyxdC1qK4NE)

  

## How it works

  

Check on dev.to to have a detailed post on how works the project.

  

### Scheduler

  

Add task to a queue with a timestamp. Determine the timestamp by looking at the frequency of access for the given data

  

### How the data is stored:

  

* Store if an region exist or not:

	* If it exists: `SADD validRegions {regionName}`

	* If it does not exist: `SADD invalidRegions {regionName}`

  

* Add the task to a sorted set: `ZADD indexationDelayed {timestamp} {regionId}`

* Add the message ID once the event finished:
    * `SET scheduler:indexationFinishedLastId {id} 0`
    * `SET scheduler:indexationCatchupLastId {id} 0`
  
  

### How the data is accessed:

* Get the last event ID finished by the worker:
    * `GET scheduler:indexationFinishedLastId`
    * `GET scheduler:indexationCatchupLastId`

* Listen to 2 stream to determine what kind of schedule it needs to do:

	* When an indexation is finished: `XREAD BLOCK 2 COUNT 1 STREAMS indexationFinished {id}`

	* When a catchup is required: `XREAD BLOCK 2 COUNT 1 STREAMS indexationCatchup {id}`

  

* Check if the region exist looking into 2 sets:

	* `SISMEMBER validRegions {regionName}`

	* `SISMEMBER invalidRegions {regionName}`

  

* It calculate the timestamp for the task using timeseries data:

	* `TS.RANGE regionFetchHistory:{regionId} {now-5minutes} {now} AGGREGATION sum 5 minutes`

	* `TS.RANGE regionFetchHistory:{regionId} {now-1hours} {now} AGGREGATION sum 1 hours`

  
  

### Delayer

  

Determine if a queued task can be started or wait until once is ready

  

#### How the data is stored:

  

* Send an event into stream `indexationAdd` when an the timestamp of a task is less or equal to now:

	`XADD indexationAdd * regionId {regionId}`

  

#### How the data is accessed:

  

* Get the first element of a sorted set with its score: 
`ZRANGE indexationDelayed 0 0 WITHSCORES`

  

### Indexer

  

Aggregate and store the data

  

### How the data is stored

  

* Store extra data that can be required for indexation if they do not already exist (eg: regionName, systemName, ...): `SET {types}:{id} {value} 0`

	* eg: `SET regions:10000032 Sinq Laison 0`

  

* Store aggregated data as json `JSON.SET denormalizedOrders:{locationId}:{typeId} {value}`

	* eg: `JSON.SET denormalizedOrders:60014692:1137 '{"regionId": 1000032, "locationId": 60014692, "typeId": 1137, "buyPrice": 100, "sellPrice": 200}'`

  

	* these data have a ttl bind to them once created: `EXPIRE denormalizedOrders:{locationId}:{typeId} 86400`

  

* Send an event to inform that indexation is finished `XADD indexationFinished * regionId {regionId}`

  

#### How the data is accessed:

  

* Listen to a stream to run indexation, ID change depending on if the worker has restarted or not (it is `O` at start, `$` once the backlog empty):
	*  `XREADGROUP GROUP indexationAddGroup {consumer-name} BLOCK 2 COUNT 1 STREAMS indexationAdd {id}`

  

* Read the extra data required for indexation : `READ {types}:{id}`

	* eg: `READ regions:10000032`

  

### Heartbeat

  

Subscribe to a pub/sub to store access count to a region

  

#### How the data is stored:

  

* Write access to a region into a timeseries key: `TS.ADD regionFetchHistory:{regionId} {now in milliseconds} 1`

  

#### How the data is accessed:

  

* Subscribe to event publish: `SUBSCRIBE apiEvent`


### Refresh

  

Subscribe to a pub/sub to determine if a catch-up is needed

  

#### How the data is stored:
  

* Send an event to a stream if a catch-up is required: `XADD indexationCatchup * regionId {regionId}`

* Store the catch-up request for 5 minutes: `SET indexationCatchupLaunch:{regionId} 300`

  

#### How the data is accessed:

* Subscribe to event publish: `SUBSCRIBE apiEvent`

* Check if a catch-up has been request in the last 5 minutes: `GET indexationCatchupLaunch:{regionId} 300`

* Check in the queued task for the next 5 minutes if the regionId is scheduled: `ZRANGEBYSCORE indexationDelayed {now} {now+5minutes}`

  

### API

  

Provide aggregated data to end user

  

### How the data is stored:

  

* Publish into a pub/sub each time the route is called: `PUBLISH apiHeartbeat {regionId}`

  

### How the data is accessed:

  

* Search in the JSON entries data that match filter provided by user

  
```
eg (with location as string):


FT.SEARCH denormalizedOrdersIdx "@locationName:(Dodixie IX Moon 20)|@systemName:(Dodixie IX Moon 20)|@regionName:(Dodixie IX Moon 20) @buyPrice:[5000000.00 10000000] @sellPrice:[6000000 20000000]" LIMIT 0 10000


eg (with location as id):

FT.SEARCH denormalizedOrdersIdx "@locationIdTags:{60011866} @buyPrice:[5000000.00 10000000] @sellPrice:[6000000 20000000]" LIMIT 0 10000
```

### CLI

Provide a CLI tool to interact with Redis for installation and warming up the application

* Creation of the index

```
FT.CREATE denormalizedOrdersIx
    ON JSON
    PREFIX 1 denormalizedOrders:
    SCHEMA
        $.regionId AS regionId NUMERIC
        $.systemId AS systemId NUMERIC
        $.locationId AS locationId NUMERIC
        $.typeId AS typeId NUMERIC
        $.buyPrice AS buyPrice NUMERIC
        $.sellPrice AS sellPrice NUMERIC
        $.buyVolume AS buyVolume NUMERIC
        $.sellVolume AS sellVolume NUMERIC
        $.locationName AS locationName TEXT
        $.systemName AS systemName TEXT
        $.regionName AS regionName TEXT
        $.typeName AS typeName TEXT
        $.locationIdTags AS locationIdTags TAG SEPARATOR ","
```

* Creation of the group stream (and creating the stream in same time) `XGROUP CREATE indexationAdd indexationAddGroup 0 MKSTREAM`

## How to run it locally?
  

### Prerequisites

* Docker & Docker-composer
* Internet connection
* GO 1.18 if you cannot use the cli provided in the release
  

### Local installation

#### 1. With redis from docker-compose
* Change `.env.docker.dist` to `.env.docker.local` (modify value inside if you plan to change the password, make sure to do it also in the docker-compose redis service)
* Change `.env.dist` to `.env.local` (change password if you changed it in the step before)
* Run `docker-compose up` 
* Use one of the executable in the release [link to release] and run the following commands:
	* Run `wall-eve-cli-{youros} install --envFile=.env.local`
	* Run `wall-eve-cli-{youros} warmup 10000032 --envFile=.env.local`
	* Run `wall-eve-cli-{youros} warmup 10000002 --envFile=.env.local`

#### 2. With redis from not docker-compose
* Change `.env.dist` to `.env.local` and change the value using your own redis address (don't forget to add the port at the end of the address `{adress}:{port}`)
* Run `docker-compose -f docker-compose-noredis.yaml up` 
* Use one of the executable in the release [link to release] and run the following commands:
	* Run `wall-eve-cli-{youros} install --envFile=.env.local`
	* Run `wall-eve-cli-{youros} warmup 10000032 --envFile=.env.local`
	* Run `wall-eve-cli-{youros} warmup 10000002 --envFile=.env.local`

**If you cannot run one of the binary, you can follow the following step (it requires go 1.18 installed)s**
* `cd backend`
* `go mod download`
* `go run cmd/cli/main.go install --env=.env.local`
* `cd backend && go run cmd/cli/main.go warmup 10000032 --env=.env.local`
* `cd backend && go run cmd/cli/main.go warmup 10000002 --env=.env.local`

It can take 2-3 minutes before the first entries are saved into the application

You can access the API at `http://127.0.0.1:1337`
You can find the swagger at `http://127.0.0.1:1338`

If you are not familiar with Eve online, you can find below some query parameters to use with the API

## Deployment

To make deploys work, you need to create free account on [Redis Cloud](https://redis.info/try-free-dev-to)
