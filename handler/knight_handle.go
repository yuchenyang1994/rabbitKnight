package hander

import (
	"net/http"
	"rabbitKnight/rabbit"

	"io/ioutil"

	"encoding/json"

	"log"

	"io/ioutil"

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

type KnightForRestart struct {
	QueueName   string `json:"queueName"`
	ProjectName string `json:"queueName"`
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
	r.JSON(200, knightProjects)
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

// DeleteKnightForQueueName ...
func StopKnightForQueueName(doneHub *rabbit.KnightDoneHub, knightMapping *rabbit.RabbitKnightMapping, r render.Render, params martini.Params) {
	queueName := params["queueName"]
	projectName := params["projectName"]
	man := knightMapping.GetmanForName(projectName, queueName)
	if man.Status {
		doneHub.StopKnightByQueueName(queueName)
		r.JSON(200, map[string]string{"message": "success"})
	} else {
		r.JSON(405, map[string]string{"message": "fail"})
	}
}

// StartKnightForQueueName
func StartKnightForQueueName(doneHub *rabbit.KnightDoneHub, config *rabbit.KnightConfigManager, knightMapping *rabbit.RabbitKnightMapping, r render.Render, params martini.Params) {
	queueName := params["queueName"]
	projectName := params["projectName"]
	man := knightMapping.GetmanForName(projectName, queueName)
	if man.Status {
		r.JSON(405, map[string]string{"message": "fail"})
	} else {
		done := make(chan struct{}, 1)
		doneHub.DoneMap[queueName] = done
		go man.RunKnight(done)
		r.JSON(405, map[string]string{"message": "success"})
	}
}

// CreateKnightForQueueName ....
func CreateKnightForQueueName(doneHub *rabbit.KnightDoneHub, config *rabbit.KnightConfigManager, knightMapping *rabbit.RabbitKnightMapping, r render.Render, parms martini.Params, req *http.Request, hub *rabbit.KnightHub) {
	projectName := parms["projectName"]
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Fatal(err)
	}
	var queueConfig rabbit.QueueConfig
	json.Unmarshal(body, &queueConfig)
	config.SaveQueuesConfig(queueConfig, projectName, queueConfig.QueueName)
	queue := config.SetProjectForName(projectName, queueConfig)
	done := make(chan struct{}, 1)
	doneHub.DoneMap[queue.QueueName] = done
	man := rabbit.NewRabbitKnightMan(queue, config.AmqpConfig, hub)
	knightMapping.SetManFormQueueConfig(queue, man)
	go man.RunKnight(done)
	r.JSON(200, map[string]string{"message": "fail"})
}

func CreateKnightForProject(req *http.Request, config *rabbit.KnightConfigManager, mapping *rabbit.RabbitKnightMapping, hub *rabbit.KnightHub, doneHub *rabbit.KnightDoneHub) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Fatal(err)
	}
	allQueues := config.LoadQueuesForJSON(body)
	for _, queueConfig := range allQueues {
		done := make(chan struct{}, 1)
		doneHub.DoneMap[queueConfig.QueueName] = done
		man := rabbit.NewRabbitKnightMan(queueConfig, config.AmqpConfig, hub)
		mapping.SetManFormQueueConfig(queueConfig, man)
		go man.RunKnight(done)
	}

}
