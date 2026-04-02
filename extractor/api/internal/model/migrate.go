package model

import "gorm.io/gorm"

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&AccountBinding{},
		&Strategy{},
		&StrategyRun{},
		&Order{},
		&Trade{},
		&Position{},
		&RiskRule{},
		&RiskAlert{},
		&DailyPnL{},
		&BacktestJob{},
		&Settlement{},
		&PredictMarket{},
		&PredictPosition{},
		&ClientAccount{},
		&MonthlyBill{},
		&NAVSnapshot{},
	)
}
