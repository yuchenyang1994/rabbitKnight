package main

import (
	"rabbitKnight/handler"

	"github.com/go-martini/martini"
)

func router(server *martini.ClassicMartini) {
	server.Get("/knights", handler.GetKnightStatus)
	server.Post("/knights", handler.CreateKnightForProject)

	server.Get("/knights/:projectName", handler.GetKnightStatusByProjectName)
	server.Delete("/knights/:projectName/:queueName", handler.StopKnightForQueueName)
	server.Put("/knights/:projectName/:queueName", handler.StartKnightForQueueName)
	server.Post("/knights/:projectName", handler.CreateKnightForProject)
}
