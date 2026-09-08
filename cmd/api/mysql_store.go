package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type MySQLGameRunStore struct {
	db *gorm.DB
}

// gameRunRecord is a database model.
type gameRunRecord struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement"`
	PlayerID        string    `gorm:"size:128;not null;index"`
	SurvivalSeconds int       `gorm:"not null"`
	Level           int       `gorm:"not null"`
	NormalKills     int       `gorm:"not null"`
	FastKills       int       `gorm:"not null"`
	TankKills       int       `gorm:"not null"`
	Result          string    `gorm:"size:16;not null;index"`
	CreatedAt       time.Time `gorm:"not null"`
}

func (gameRunRecord) TableName() string {
	return "game_runs"
}

func NewMySQLGameRunStore(
	ctx context.Context,
	dsn string,
) (*MySQLGameRunStore, error) {
	db, err := gorm.Open(
		mysql.Open(dsn),
		&gorm.Config{},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"open MySQL connection: %w",
			err,
		)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf(
			"get underlying SQL database: %w",
			err,
		)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf(
			"ping MySQL: %w",
			err,
		)
	}

	if err := db.WithContext(ctx).
		AutoMigrate(&gameRunRecord{}); err != nil {
		return nil, fmt.Errorf(
			"migrate game_runs table: %w",
			err,
		)
	}

	return &MySQLGameRunStore{
		db: db,
	}, nil
}

func (s *MySQLGameRunStore) Create(
	ctx context.Context,
	run GameRun,
) (GameRun, error) {
	record := gameRunRecord{
		PlayerID:        run.PlayerID,
		SurvivalSeconds: run.SurvivalSeconds,
		Level:           run.Level,
		NormalKills:     run.NormalKills,
		FastKills:       run.FastKills,
		TankKills:       run.TankKills,
		Result:          run.Result,
	}

	if err := s.db.
		WithContext(ctx).
		Create(&record).
		Error; err != nil {
		return GameRun{}, fmt.Errorf(
			"insert game run: %w",
			err,
		)
	}

	run.ID = strconv.FormatUint(record.ID, 10)

	return run, nil
}

func (s *MySQLGameRunStore) All(
	ctx context.Context,
) ([]GameRun, error) {
	var records []gameRunRecord

	if err := s.db.
		WithContext(ctx).
		Order("id ASC").
		Find(&records).
		Error; err != nil {
		return nil, fmt.Errorf(
			"query game runs: %w",
			err,
		)
	}

	runs := make([]GameRun, 0, len(records))

	for _, record := range records {
		runs = append(runs, GameRun{
			ID: strconv.FormatUint(
				record.ID,
				10,
			),
			PlayerID:        record.PlayerID,
			SurvivalSeconds: record.SurvivalSeconds,
			Level:           record.Level,
			NormalKills:     record.NormalKills,
			FastKills:       record.FastKills,
			TankKills:       record.TankKills,
			Result:          record.Result,
		})
	}

	return runs, nil
}

func (s *MySQLGameRunStore) Stats(
	ctx context.Context,
) (GameRunStats, error) {
	var stats GameRunStats

	err := s.db.WithContext(ctx).
		Model(&gameRunRecord{}).
		Select(`
			COUNT(*) AS total_runs,
			COALESCE(MAX(survival_seconds), 0) AS best_survival_seconds,
			COALESCE(MAX(level), 0) AS highest_level,
			COALESCE(SUM(normal_kills), 0) AS total_normal_kills,
			COALESCE(SUM(fast_kills), 0) AS total_fast_kills,
			COALESCE(SUM(tank_kills), 0) AS total_tank_kills,
			COALESCE(
				SUM(normal_kills + fast_kills + tank_kills),
				0
			) AS total_kills
		`).
		Scan(&stats).Error
	if err != nil {
		return GameRunStats{}, fmt.Errorf(
			"calculate game run stats: %w",
			err,
		)
	}

	return stats, nil
}
