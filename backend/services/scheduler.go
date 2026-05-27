package services

import (
	"time"

	"living-recorder/backend/models"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type SchedulerService struct {
	db       *gorm.DB
	cron     *cron.Cron
	recorder *RecorderService
	tasks    map[uint]cron.EntryID
}

func NewSchedulerService(db *gorm.DB, recorder *RecorderService) *SchedulerService {
	return &SchedulerService{
		db:       db,
		cron:     cron.New(cron.WithSeconds()),
		recorder: recorder,
		tasks:    make(map[uint]cron.EntryID),
	}
}

func (s *SchedulerService) Start() {
	s.cron.Start()
	s.loadTasks()
}

func (s *SchedulerService) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

func (s *SchedulerService) loadTasks() {
	var tasks []models.RecordTask
	s.db.Where("enabled = ? AND schedule_cron != ''", true).Preload("Stream").Find(&tasks)
	for _, task := range tasks {
		s.AddTask(&task)
	}
}

func (s *SchedulerService) AddTask(task *models.RecordTask) error {
	if task.ScheduleCron == "" {
		return nil
	}

	taskID := task.ID
	duration := task.ScheduleDuration

	entryID, err := s.cron.AddFunc(task.ScheduleCron, func() {
		s.recorder.Start(task.StreamID, task)
		if duration > 0 {
			go func() {
				time.Sleep(time.Duration(duration) * time.Second)
				s.recorder.Stop(task.StreamID)
			}()
		}
	})
	if err != nil {
		return err
	}

	s.tasks[taskID] = entryID
	return nil
}

func (s *SchedulerService) RemoveTask(taskID uint) {
	if entryID, ok := s.tasks[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.tasks, taskID)
	}
}

func (s *SchedulerService) ReloadTask(task *models.RecordTask) {
	s.RemoveTask(task.ID)
	if task.Enabled && task.ScheduleCron != "" {
		s.AddTask(task)
	}
}
