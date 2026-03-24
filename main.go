package main

func main() {
	// 1. Загружаем конфиг
	cfg := LoadConfig()

	// 2. Создаем UI
	ui := NewUI(&cfg)

	// 3. Запускаем прокси в фоне
	go StartProxy(&cfg, ui)

	// 4. Запускаем окно
	ui.Window.ShowAndRun()
}
