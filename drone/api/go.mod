module starclaw.net/drone/api

go 1.22

require (
	github.com/gin-contrib/cors v1.7.3
	github.com/gin-gonic/gin v1.10.0
	starclaw.net/pheromone/sdk v0.0.0
)

replace starclaw.net/pheromone/sdk => ./pheromone-sdk
