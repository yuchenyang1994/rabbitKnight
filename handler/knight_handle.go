package hander

import (
	"rabbitKnight/rabbit"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

type KnightProjects struct {
	Projects []KnightProjects `json:"projects"`
}

type KnightProject struct {
	ProjectName string          `json:"projectName"`
	Knights     []KnightForJson `json:"knights"`
}

type KnightForJson struct {
	QueueName string `json:"queueName"`
	Status    bool   `json:"status"`
}

// GetKnightStatus ...
func GetKnightStatus(knightMapping *rabbit.RabbitKnightMapping, r render.Render) {
	knights := knightMapping.GetAllMans()
	knightProjects := []KnightProject{}
	for projectName, knights := range knights {
		knightForJsons := []KnightForJson{}
		for queueName, knight := range knights {
			knightForJsons = append(knightForJsons, KnightForJson{queueName, knight.Status})
		}
		knightProjects = append(knightProjects, KnightProject{projectName, knightForJsons})
	}
	r.JSON(200, knights)

}

// GetKnightStatusByProjectName ...
func GetKnightStatusByProjectName(knightMapping *rabbit.RabbitKnightMapping, r render.Render, params martini.Params) {
	projectName := params["projectName"]
	knights := knightMapping.GetMansForProjectName(projectName)
	knightForJsons := []KnightForJson{}
	for queueName, knight := range knights {
		knightForJsons = append(knightForJsons, KnightForJson{queueName, knight.Status})
	}
	projects := KnightProject{projectName, knightForJsons}
	r.JSON(200, projects)
}

// StopKnightByQueueName ...
func StopKnightForQueueName(doneHub *rabbit.KnightDoneHub, r render.Render, params martini.Params) {
	queueName := params["queueName"]
	doneHub.StopKnightByQueueName(queueName)
	r.JSON(200, map[string]string{"message": "success"})
}
