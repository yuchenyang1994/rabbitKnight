package handler

import (
	"rabbitKnight/rabbit"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

func GetAllConfigs(configs *rabbit.KnightConfigManager, r render.Render) {
	projects := configs.Configs.Projects
	r.JSON(200, projects)
}

func GetAllConfigsByProjectName(configs *rabbit.KnightConfigManager, r render.Render, params martini.Params) {
	projectName := params["projectName"]
	projects := configs.Configs.Projects
	project := projects[projectName]
	r.JSON(200, project)
}

func GetConfigsByQueue(configs *rabbit.KnightConfigManager, r render.Render, params martini.Params) {
	projectName := params["projectName"]
	queuename := params["queueName"]
	queueConfig := configs.GetQueueConfig(queuename, projectName)
	r.JSON(200, queueConfig)
}
