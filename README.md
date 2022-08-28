
# Wall-Eve

  

Wall-Eve is an API that provide aggregated data from the market of the game Eve-Online.

  

Eve-Online is a MMORPG where trading is an important aspect of the game. The developers provide a lot of endpoints to help the development of third parties tool but the endpoint for the market does not provide aggregated data, meaning that most of the developers creating market application have to pull the data, aggregate it and then work with it.

  

Wall-Eve is a possible solution to this problem. It aggregate the data and let people retrieve informations using the API.

The application will refresh the data regurarly to provide up-to-data informations.

  
  

[Insert app screenshots](https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax#uploading-assets)

  

# Overview video (Optional)

  

Here's a short video that explains the project and how it uses Redis:

  

[Insert your own video here, and remove the one below]

  

[![Embed your YouTube video](https://i.ytimg.com/vi/vyxdC1qK4NE/maxresdefault.jpg)](https://www.youtube.com/watch?v=vyxdC1qK4NE)

  

## How it works

  

Check on dev.to to have a detailled post on how works the project.

  

### Scheduler

  

Add task to a queue with a timestamp. Determine the timestamp by looking at the frequency of access for the given data

  

### How the data is stored:

  

* Store if an region exist or not:

	* If it exist: `SADD validRegions {regionName}`

	* If it does not exist: `SADD invalidRegions {regionName}`

  

* Add the task to a sorted set: `ZADD indexationDelayed {timestamp} {regionId}`

* Add the message id once the event finished:
    * `SET scheduler:indexationFinishedLastId {id} 0`
    * `SET scheduler:indexationCatchupLastId {id} 0`
  
  

### How the data is accessed:

* Get the last event id finished by the worker:
    * `GET scheduler:indexationFinishedLastId`
    * `GET scheduler:indexationCatchupLastId`

* Listen to 2 stream to determine what kind of schedule it need to do:

	* When an indexation is finished: `XREAD BLOCK 2 COUNT 1 STREAMS indexationFinished {id}`

	* When a catchup is required: `XREAD BLOCK 2 COUNT 1 STREAMS indexationCatchup {id}`

  

* Check if the region exist looking into 2 sets:

	* `SISMEMBER validRegions {regionName}`

	* `SISMEMBER invalidRegions {regionName}`

  

* It calculate the timestamp for the task using timeseries data:

	* `TS.RANGE regionFetchHistory:{regionId} {now-5minutes} {now} AGGREGATION sum 5 minutes`

	* `TS.RANGE regionFetchHistory:{regionId} {now-1hours} {now} AGGREGATION sum 1 hours`

  
  

### Delayer

  

Determine if a queued task can be runned or wait until once is ready

  

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

  

	* this data have a ttl bind to them once created: `EXPIRE denormalizedOrders:{locationId}:{typeId} 86400`

  

* Send an event to inform that indexation is finished `XADD indexationFinished * regionId {regionId}`

  

#### How the data is accessed:

  

* Listen to a stream to run indexation, id change depending if the worker has restarted or not (it is `O` at start, `$` once the backlog empty):
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

  

Subscribe to a pub/sub to determine if a catchup is needed

  

#### How the data is stored:
  

* Send an event to a stream if a catchup is required: `XADD indexationCatchup * regionId {regionId}`

* Store the catchup request for 5 minutes: `SET indexationCatchupLaunch:{regionId} 300`

  

#### How the data is accessed:

* Subscribe to event publish: `SUBSCRIBE apiEvent`

* Check if a catchup has been request in the last 5 minutes: `GET indexationCatchupLaunch:{regionId} 300`

* Check in the queued task for the next 5 minutes if the regionId is scheduled: `ZRANGEBYSCORE indexationDelayed {now} {now+5minutes}`

  

### API

  

Provide aggregated data to end user

  

### How the data is stored:

  

* Publish into a pub/sub each time the route is called: `PUBLISH apiHeartbeat {regionId}`

  

### How the data is accessed:

  

* Search in the JSON entries data that match filter provided by user

  

	eg (with location as string):

```

FT.SEARCH denormalizedOrdersIdx @locationName:(Dodixie IX Moon 20)|@systemName:(Dodixie IX Moon 20)|@regionName:(Dodixie IX Moon 20) @buyPrice:[5000000.00 10000000] @sellPrice:[6000000 20000000] LIMIT 0 10000

```

	eg (with location as id):

```
FT.SEARCH denormalizedOrdersIdx @locationIdsTag:{60011866} @buyPrice:[5000000.00 10000000] @sellPrice:[6000000 20000000] LIMIT 0 10000
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
* GO 1.18
* Internet connection
* A remote redis server with at least 200MB memory
  

### Local installation

* Change `.env.dist` to `.env` and modify value inside
* Use one of the executable in the release [link to release] and run the following commands:
    * `wall-eve-cli-{youros} install --envFile=$pathToYourEnvFile`
* Run `docker-compose up`
* Run `wall-eve-cli warmup 10000032 --envFile=$pathToYourEnvFile`

The process will now start and pull the data

**If you cannot run one of the binary, you can follow the following step (it requires go 1.18 installed)s**
* Change `.env.dist` to `.env` and modify value inside
* `cd backend`
* `go mod download`
* `go run cmd/cli/main.go install --env=$pathToYourEnvFile`
* `cd .. & docker-compose up`
* `cd backend && go run cmd/cli/main.go warmup 10000032 --env=$pathToYourEnvFile`

You can access the API on `http://127.0.0.1:1337`

If you are not familiar with Eve online, you can find below some query parameters to use with the API

## Deployment

To make deploys work, you need to create free account on [Redis Cloud](https://redis.info/try-free-dev-to)
