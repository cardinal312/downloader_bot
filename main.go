// package main

// import (
// 	"bufio"
// 	"fmt"
// 	"log"
// 	"math"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"regexp"
// 	"strconv"
// 	"strings"
// 	"sync"
// 	"time"

// 	"gopkg.in/telebot.v3"
// 	"gopkg.in/yaml.v3"
// )

// type Config struct {
// 	Telegram struct {
// 		Token string `yaml:"token"`
// 	} `yaml:"telegram"`
// 	Download struct {
// 		OutputDir   string `yaml:"output_dir"`
// 		MaxSizeMB   int    `yaml:"max_size_mb"`
// 		CookiesFile string `yaml:"cookies_file"`
// 	} `yaml:"download"`
// }

// var (
// 	cfg       Config
// 	userState sync.Map
// 	btnMP4    = telebot.Btn{Unique: "mp4"}
// 	btnMP3    = telebot.Btn{Unique: "mp3"}
// )

// func init() {
// 	if err := loadConfig(); err != nil {
// 		log.Fatalf("Failed to load config: %v", err)
// 	}
// 	if err := checkDependencies(); err != nil {
// 		log.Fatalf("Dependencies check failed: %v", err)
// 	}
// }

// func loadConfig() error {
// 	data, err := os.ReadFile("config.yaml")
// 	if err != nil {
// 		return fmt.Errorf("error reading config: %w", err)
// 	}

// 	if err := yaml.Unmarshal(data, &cfg); err != nil {
// 		return fmt.Errorf("error parsing config: %w", err)
// 	}

// 	if cfg.Telegram.Token == "" {
// 		return fmt.Errorf("telegram token not configured")
// 	}

// 	if cfg.Download.OutputDir == "" {
// 		cfg.Download.OutputDir = "downloads"
// 	}

// 	if cfg.Download.MaxSizeMB == 0 {
// 		cfg.Download.MaxSizeMB = 50
// 	}

// 	if err := os.MkdirAll(cfg.Download.OutputDir, 0755); err != nil {
// 		return fmt.Errorf("error creating downloads dir: %w", err)
// 	}

// 	return nil
// }

// func checkDependencies() error {
// 	if _, err := exec.LookPath("yt-dlp"); err != nil {
// 		return fmt.Errorf("yt-dlp not found: %w", err)
// 	}
// 	if _, err := exec.LookPath("ffmpeg"); err != nil {
// 		return fmt.Errorf("ffmpeg not found: %w", err)
// 	}
// 	return nil
// }

// func downloadMedia(url, format string, chatID int64, b *telebot.Bot) (string, error) {
// 	// Генерируем уникальное имя файла с наносекундами
// 	timestamp := time.Now().UnixNano()
// 	outputTemplate := filepath.Join(cfg.Download.OutputDir, fmt.Sprintf("%d.%%(ext)s", timestamp))
// 	progressChan := make(chan float64, 10)

// 	msg, err := b.Send(telebot.ChatID(chatID), "🔄 Подготовка к загрузке...")
// 	if err != nil {
// 		close(progressChan)
// 		return "", fmt.Errorf("failed to send initial message: %w", err)
// 	}

// 	// Запускаем горутину для отображения прогресса
// 	go func() {
// 		showProgress(b, chatID, msg, progressChan)
// 		close(progressChan)
// 	}()

// 	args := []string{
// 		"--newline",
// 		"--no-check-certificate",
// 		"--force-ipv4",
// 		"--geo-bypass",
// 		"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
// 		"-o", outputTemplate,
// 	}

// 	switch format {
// 	case "mp4":
// 		args = append(args,
// 			"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best",
// 			"--merge-output-format", "mp4",
// 			"--recode-video", "mp4",
// 			"--postprocessor-args", "ffmpeg:-c:v libx264 -preset fast -crf 22 -c:a aac -b:a 128k -movflags +faststart",
// 		)
// 	case "mp3":
// 		args = append(args,
// 			"-x",
// 			"--audio-format", "mp3",
// 			"--audio-quality", "0",
// 		)
// 	default:
// 		return "", fmt.Errorf("unsupported format: %s", format)
// 	}

// 	if cfg.Download.CookiesFile != "" {
// 		if _, err := os.Stat(cfg.Download.CookiesFile); err == nil {
// 			args = append([]string{"--cookies", cfg.Download.CookiesFile}, args...)
// 		}
// 	}

// 	args = append(args, url)

// 	cmd := exec.Command("yt-dlp", args...)
// 	stdout, err := cmd.StdoutPipe()
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
// 	}
// 	cmd.Stderr = cmd.Stdout

// 	// Горутина для сканирования вывода
// 	done := make(chan struct{})
// 	go func() {
// 		defer close(done)
// 		scanner := bufio.NewScanner(stdout)
// 		for scanner.Scan() {
// 			line := scanner.Text()
// 			if percent, err := parseProgress(line); err == nil {
// 				select {
// 				case progressChan <- percent:
// 				case <-done:
// 					return
// 				}
// 			}
// 		}
// 	}()

// 	if err := cmd.Start(); err != nil {
// 		return "", fmt.Errorf("download failed to start: %w", err)
// 	}

// 	if err := cmd.Wait(); err != nil {
// 		return "", fmt.Errorf("download failed: %w", err)
// 	}

// 	// Ждем завершения сканирования вывода
// 	<-done

// 	// Найдём результат по расширению
// 	pattern := filepath.Join(cfg.Download.OutputDir, fmt.Sprintf("%d.*", timestamp))
// 	matches, err := filepath.Glob(pattern)
// 	if err != nil {
// 		return "", fmt.Errorf("ошибка при поиске файла: %w", err)
// 	}
// 	if len(matches) == 0 {
// 		return "", fmt.Errorf("файл не найден после загрузки")
// 	}

// 	return matches[0], nil
// }

// func parseProgress(line string) (float64, error) {
// 	re := regexp.MustCompile(`\[download\]\s+(\d+\.\d+)%`)
// 	matches := re.FindStringSubmatch(line)
// 	if len(matches) < 2 {
// 		return 0, fmt.Errorf("no progress found")
// 	}
// 	return strconv.ParseFloat(matches[1], 64)
// }

// func showProgress(b *telebot.Bot, chatID int64, msg *telebot.Message, progressChan <-chan float64) {
// 	var (
// 		lastPercent float64
// 		lastUpdate  time.Time
// 	)

// 	for percent := range progressChan {
// 		if time.Since(lastUpdate) > time.Second && math.Abs(percent-lastPercent) > 1 {
// 			progressBar := strings.Repeat("▓", int(percent/10)) + strings.Repeat("░", 10-int(percent/10))
// 			emoji := getProgressEmoji(percent)
// 			text := fmt.Sprintf("%s Загрузка: [%s] %.1f%%", emoji, progressBar, percent)

// 			if _, err := b.Edit(msg, text); err != nil {
// 				log.Printf("Failed to update progress: %v", err)
// 				continue
// 			}

// 			lastUpdate = time.Now()
// 			lastPercent = percent
// 		}
// 	}
// }

// func getProgressEmoji(percent float64) string {
// 	switch {
// 	case percent < 30:
// 		return "🐢"
// 	case percent < 70:
// 		return "🚶"
// 	default:
// 		return "🏃"
// 	}
// }

// func handleFormatSelection(c telebot.Context, format string) error {
// 	userID := c.Chat().ID
// 	val, ok := userState.Load(userID)
// 	if !ok {
// 		return c.Send("❌ Ссылка устарела. Отправьте новую.")
// 	}
// 	url := val.(string)
// 	log.Printf("FILEEEEEEEE----%s", url)
// 	filename, err := downloadMedia(url, format, userID, c.Bot())
// 	log.Printf("FILEEEEEEEE----%s", filename)
// 	if err != nil {
// 		log.Printf("Download error: %v", err)
// 		return c.Send("❌ Произошла ошибка при загрузке файла.")
// 	}
// 	defer func() {
// 		if err := os.Remove(filename); err != nil {
// 			log.Printf("Failed to remove temp file: %v", err)
// 		}
// 	}()

// 	var file interface{}
// 	switch format {
// 	case "mp4":
// 		file = &telebot.Video{
// 			File:     telebot.FromDisk(filename),
// 			FileName: "video.mp4",
// 		}
// 	case "mp3":
// 		file = &telebot.Audio{
// 			File:     telebot.FromDisk(filename),
// 			FileName: "audio.mp3",
// 		}
// 	default:
// 		return c.Send("❌ Неподдерживаемый формат")
// 	}

// 	if err = c.Send(file); err != nil {
// 		return c.Send("❌ Не удалось отправить файл")
// 	}

// 	userState.Delete(userID)
// 	return nil
// }

// func main() {
// 	pref := telebot.Settings{
// 		Token:  cfg.Telegram.Token,
// 		Poller: &telebot.LongPoller{Timeout: 30 * time.Second},
// 	}

// 	pref.Token = "8081348600:AAFPYUSrmIItTZXbZsrkpTf97aaU6hYUSIk"

// 	b, err := telebot.NewBot(pref)
// 	if err != nil {
// 		log.Fatalf("Failed to create bot: %v", err)
// 	}

// 	// Глобальные обработчики кнопок
// 	b.Handle(&btnMP4, func(c telebot.Context) error {
// 		return handleFormatSelection(c, "mp4")
// 	})
// 	b.Handle(&btnMP3, func(c telebot.Context) error {
// 		return handleFormatSelection(c, "mp3")
// 	})

// 	b.Handle("/start", func(c telebot.Context) error {
// 		return c.Send("Привет! Отправь мне ссылку на видео, и я скачаю его для тебя.")
// 	})

// 	b.Handle(telebot.OnText, func(c telebot.Context) error {
// 		url := c.Text()
// 		if url == "" {
// 			return c.Send("❌ Пожалуйста, отправьте ссылку на видео.")
// 		}

// 		userID := c.Chat().ID
// 		userState.Store(userID, url)

// 		selector := &telebot.ReplyMarkup{}
// 		btnRowMP4 := selector.Data("🎬 Видео (MP4)", btnMP4.Unique)
// 		btnRowMP3 := selector.Data("🎵 Аудио (MP3)", btnMP3.Unique)
// 		selector.Inline(selector.Row(btnRowMP4, btnRowMP3))

// 		return c.Send("Выберите формат:", selector)
// 	})

// 	// b.Handle(telebot.OnError, func(err error, c telebot.Context) {
// 	// 	log.Printf("Telegram error: %v", err)
// 	// })

// 	log.Println("Бот запущен...")
// 	b.Start()
// }

package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/telebot.v3"
	"gopkg.in/yaml.v3"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Config struct {
	Telegram struct {
		Token string `yaml:"token"`
	} `yaml:"telegram"`
	Download struct {
		OutputDir   string `yaml:"output_dir"`
		MaxSizeMB   int    `yaml:"max_size_mb"`
		CookiesFile string `yaml:"cookies_file"`
	} `yaml:"download"`
}

var (
	cfg       Config
	userState sync.Map
	btnMP4    = telebot.Btn{Unique: "mp4"}
	btnMP3    = telebot.Btn{Unique: "mp3"}
)

func init() {
	if err := loadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if err := checkDependencies(); err != nil {
		log.Fatalf("Dependencies check failed: %v", err)
	}
}

func loadConfig() error {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		return fmt.Errorf("error reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("error parsing config: %w", err)
	}

	if cfg.Telegram.Token == "" {
		return fmt.Errorf("telegram token not configured")
	}

	if cfg.Download.OutputDir == "" {
		cfg.Download.OutputDir = "downloads"
	}

	if cfg.Download.MaxSizeMB == 0 {
		cfg.Download.MaxSizeMB = 50
	}

	if err := os.MkdirAll(cfg.Download.OutputDir, 0755); err != nil {
		return fmt.Errorf("error creating downloads dir: %w", err)
	}

	return nil
}

func checkDependencies() error {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return fmt.Errorf("yt-dlp not found: %w", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found: %w", err)
	}
	return nil
}

func downloadMedia(url string, format string, chatID int64, b *telebot.Bot) (string, error) {
	// Создаем уникальное имя файла с временной меткой
	filename := filepath.Join(cfg.Download.OutputDir, fmt.Sprintf("%d.%s", time.Now().Unix(), format))

	// Убедимся, что директория существует
	if err := os.MkdirAll(cfg.Download.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать директорию: %w", err)
	}

	// Аргументы для yt-dlp
	args := []string{
		"--newline",
		"--no-check-certificate",
		"--force-ipv4",
		"--geo-bypass",
		"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"-o", filename,
	}

	// Добавляем параметры в зависимости от формата
	if format == "mp4" {
		args = append(args,
			"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best",
			"--merge-output-format", "mp4",
			"--recode-video", "mp4",
		)
	} else { // mp3
		args = append(args,
			"-x",
			"--audio-format", "mp3",
			"--audio-quality", "0",
		)
	}

	// Добавляем cookies если они есть
	if cfg.Download.CookiesFile != "" {
		if _, err := os.Stat(cfg.Download.CookiesFile); err == nil {
			args = append([]string{"--cookies", cfg.Download.CookiesFile}, args...)
		}
	}

	// Добавляем URL в конец аргументов
	args = append(args, url)

	// Запускаем команду
	cmd := exec.Command("yt-dlp", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ошибка загрузки: %w\nВывод: %s", err, string(output))
	}

	// Проверяем что файл действительно создан
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// Пробуем найти файл с другим расширением
		if matches, _ := filepath.Glob(filename[:len(filename)-4] + ".*"); len(matches) > 0 {
			return matches[0], nil
		}
		return "", fmt.Errorf("файл не найден после загрузки. Проверьте доступное место на диске")
	}

	return filename, nil
}

// func downloadMedia(url, format string, chatID int64, b *telebot.Bot) (string, error) {
// 	outputTemplate := filepath.Join(cfg.Download.OutputDir, fmt.Sprintf("%d.%%(ext)s", time.Now().Unix()))
// 	progressChan := make(chan float64, 10)
// 	defer close(progressChan)

// 	// msg, err := b.Send(chatID, "🔄 Подготовка к загрузке...")
// 	// if err != nil {
// 	// 	return "", fmt.Errorf("failed to send initial message: %w", err)
// 	// }

// 	// recipient := &telebot.User{ID: chatID}
// 	// msg, err := b.Send(recipient, "🔄 Подготовка к загрузке...")

// 	msg, err := b.Send(telebot.ChatID(chatID), "🔄 Подготовка к загрузке...")

// 	go showProgress(b, chatID, msg, progressChan)

// 	args := []string{
// 		"--newline",
// 		"--no-check-certificate",
// 		"--force-ipv4",
// 		"--geo-bypass",
// 		"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
// 		"-o", outputTemplate,
// 	}

// 	switch format {
// 	case "mp4":
// 		args = append(args,
// 			"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best",
// 			"--merge-output-format", "mp4",
// 			"--recode-video", "mp4",
// 			"--postprocessor-args", "ffmpeg:-c:v libx264 -preset fast -crf 22 -c:a aac -b:a 128k -movflags +faststart",
// 		)
// 	case "mp3":
// 		args = append(args,
// 			"-x",
// 			"--audio-format", "mp3",
// 			"--audio-quality", "0",
// 		)
// 	default:
// 		return "", fmt.Errorf("unsupported format: %s", format)
// 	}

// 	if cfg.Download.CookiesFile != "" {
// 		if _, err := os.Stat(cfg.Download.CookiesFile); err == nil {
// 			args = append([]string{"--cookies", cfg.Download.CookiesFile}, args...)
// 		}
// 	}

// 	args = append(args, url)

// 	cmd := exec.Command("yt-dlp", args...)
// 	stdout, err := cmd.StdoutPipe()
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
// 	}
// 	cmd.Stderr = cmd.Stdout

// 	go func() {
// 		scanner := bufio.NewScanner(stdout)
// 		for scanner.Scan() {
// 			line := scanner.Text()
// 			if percent, err := parseProgress(line); err == nil {
// 				select {
// 				case progressChan <- percent:
// 				default:
// 				}
// 			}
// 		}
// 	}()

// 	if err := cmd.Start(); err != nil {
// 		return "", fmt.Errorf("download failed to start: %w", err)
// 	}

// 	if err := cmd.Wait(); err != nil {
// 		return "", fmt.Errorf("download failed: %w", err)
// 	}

// 	// Найдём результат по расширению
// 	matches, _ := filepath.Glob(filepath.Join(cfg.Download.OutputDir, fmt.Sprintf("%d.*", time.Now().Unix())))
// 	if len(matches) == 0 {
// 		return "", fmt.Errorf("файл не найден после загрузки")
// 	}

// 	return matches[0], nil
// }

func parseProgress(line string) (float64, error) {
	re := regexp.MustCompile(`\[download\]\s+(\d+\.\d+)%`)
	matches := re.FindStringSubmatch(line)
	if len(matches) < 2 {
		return 0, fmt.Errorf("no progress found")
	}
	return strconv.ParseFloat(matches[1], 64)
}

func showProgress(b *telebot.Bot, chatID int64, msg *telebot.Message, progressChan <-chan float64) {
	var (
		lastPercent float64
		lastUpdate  time.Time
	)

	for percent := range progressChan {
		if time.Since(lastUpdate) > time.Second && math.Abs(percent-lastPercent) > 1 {
			progressBar := strings.Repeat("▓", int(percent/10)) + strings.Repeat("░", 10-int(percent/10))
			emoji := getProgressEmoji(percent)
			text := fmt.Sprintf("%s Загрузка: [%s] %.1f%%", emoji, progressBar, percent)

			if _, err := b.Edit(msg, text); err != nil {
				log.Printf("Failed to update progress: %v", err)
				continue
			}

			lastUpdate = time.Now()
			lastPercent = percent
		}
	}
}

func getProgressEmoji(percent float64) string {
	switch {
	case percent < 30:
		return "🐢"
	case percent < 70:
		return "🚶"
	default:
		return "🏃"
	}
}

func handleFormatSelection(c telebot.Context, format string) error {
	userID := c.Chat().ID
	val, ok := userState.Load(userID)

	url := val.(string)

	if !ok {
		return c.Send("❌ Ссылка устарела. Отправьте новую.")
	}

	msg, err := c.Bot().Send(c.Chat(), "⏳ Начинаю загрузку...")
	if err != nil {
		return err
	}

	filename, err := downloadMedia(url, format, c.Chat().ID, c.Bot())
	if err != nil {
		c.Bot().Edit(msg, "❌ Ошибка: "+err.Error())
		return nil
	}
	defer func() {
		if err := os.Remove(filename); err != nil {
			log.Printf("Ошибка удаления временного файла: %v", err)
		}
	}()

	// Проверяем размер файла
	fileInfo, err := os.Stat(filename)
	if err != nil {
		c.Bot().Edit(msg, "❌ Ошибка проверки файла")
		return err
	}

	if fileInfo.Size() > int64(cfg.Download.MaxSizeMB)*1024*1024 {
		c.Bot().Edit(msg, fmt.Sprintf("❌ Файл слишком большой (%.2f MB)", float64(fileInfo.Size())/1024/1024))
		return nil
	}

	c.Bot().Edit(msg, "✅ Загрузка завершена! Отправляю файл...")

	var file interface{}
	if format == "mp4" {
		file = &telebot.Video{
			File:     telebot.FromDisk(filename),
			FileName: "video.mp4",
			MIME:     "video/mp4",
		}
	} else {
		file = &telebot.Audio{
			File:     telebot.FromDisk(filename),
			FileName: "audio.mp3",
			MIME:     "audio/mpeg",
		}
	}

	if _, err := c.Bot().Send(c.Chat(), file); err != nil {
		c.Bot().Edit(msg, "❌ Ошибка отправки файла")
		return err
	}

	c.Bot().Delete(msg)
	return nil
}

func checkDiskSpace() error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(cfg.Download.OutputDir, &stat); err != nil {
		return fmt.Errorf("ошибка проверки диска: %w", err)
	}

	// Минимум 100MB свободного места
	free := stat.Bavail * uint64(stat.Bsize)
	if free < 100*1024*1024 {
		return fmt.Errorf("недостаточно места на диске (доступно %.2f MB)", float64(free)/1024/1024)
	}
	return nil
}

func main() {

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN is not set")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Бот запущен: @%s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет, я работаю!")
			bot.Send(msg)
		}
	}

	/////
	if err := checkDiskSpace(); err != nil {
		log.Fatal(err)
	}

	pref := telebot.Settings{
		Token:  cfg.Telegram.Token,
		Poller: &telebot.LongPoller{Timeout: 30 * time.Second},
	}

	pref.Token = "8081348600:AAFPYUSrmIItTZXbZsrkpTf97aaU6hYUSIk"

	b, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// Глобальные обработчики кнопок
	b.Handle(&btnMP4, func(c telebot.Context) error {
		return handleFormatSelection(c, "mp4")
	})
	b.Handle(&btnMP3, func(c telebot.Context) error {
		return handleFormatSelection(c, "mp3")
	})

	b.Handle("/start", func(c telebot.Context) error {
		return c.Send("Привет! Отправь мне ссылку на видео, и я скачаю его для тебя.")
	})

	b.Handle(telebot.OnText, func(c telebot.Context) error {
		url := c.Text()
		if url == "" {
			return c.Send("❌ Пожалуйста, отправьте ссылку на видео.")
		}

		userID := c.Chat().ID
		userState.Store(userID, url)

		selector := &telebot.ReplyMarkup{}
		btnRowMP4 := selector.Data("🎬 Видео (MP4)", btnMP4.Unique)
		btnRowMP3 := selector.Data("🎵 Аудио (MP3)", btnMP3.Unique)
		selector.Inline(selector.Row(btnRowMP4, btnRowMP3))

		return c.Send("Выберите формат:", selector)
	})

	// b.Handle(telebot.OnError, func(err error, c telebot.Context) {
	// 	log.Printf("Telegram error: %v", err)
	// })

	log.Println("Бот запущен...")
	b.Start()

	for update := range updates {
		if update.Message != nil {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет!")
			bot.Send(msg)
		}
	}
}

/// alternativ
// package main

// import (
// 	"fmt"
// 	"log"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"strings"
// 	"time"

// 	"gopkg.in/telebot.v3"
// )

// type Config struct {
// 	Telegram struct {
// 		Token string `yaml:"token"`
// 	} `yaml:"telegram"`
// 	Download struct {
// 		OutputDir   string `yaml:"output_dir"`
// 		MaxSizeMB   int    `yaml:"max_size_mb"`
// 		CookiesFile string `yaml:"cookies_file"`
// 	} `yaml:"download"`
// }

// var (
// 	cfg       Config
// 	userState = make(map[int64]string) // Храним состояние пользователя
// )

// // ... (остальные функции init, loadConfig, checkDependencies остаются без изменений)

// func downloadMedia(url string, format string) (string, error) {
// 	filename := filepath.Join(cfg.Download.OutputDir, fmt.Sprintf("%d.%s", time.Now().Unix(), format))

// 	args := []string{
// 		"--no-check-certificate",
// 		"--force-ipv4",
// 		"--geo-bypass",
// 		"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
// 		"-o", filename,
// 		url,
// 	}

// 	if format == "mp4" {
// 		args = append(args,
// 			"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best",
// 			"--merge-output-format", "mp4",
// 			"--recode-video", "mp4",
// 			"--postprocessor-args", "ffmpeg:-c:v libx264 -preset fast -crf 22 -c:a aac -b:a 128k -movflags +faststart",
// 		)
// 	} else { // mp3
// 		args = append(args,
// 			"-x", // Извлечь аудио
// 			"--audio-format", "mp3",
// 			"--audio-quality", "0", // Лучшее качество
// 		)
// 	}

// 	if cfg.Download.CookiesFile != "" {
// 		args = append([]string{"--cookies", cfg.Download.CookiesFile}, args...)
// 	}

// 	cmd := exec.Command("yt-dlp", args...)
// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return "", fmt.Errorf("download failed: %v\n%s", err, string(output))
// 	}

// 	return filename, nil
// }

// func main() {
// 	// Добавляем проверку токена перед запуском
// 	// if cfg.Telegram.Token == "" || !strings.Contains(cfg.Telegram.Token, ":") {
// 	// 	log.Fatal("Invalid Telegram token format. It should look like '123456789:AAFmG...'")
// 	// }

// 	cfg.Telegram.Token = "8081348600:AAFPYUSrmIItTZXbZsrkpTf97aaU6hYUSIk"

// 	pref := telebot.Settings{
// 		Token:  cfg.Telegram.Token,
// 		Poller: &telebot.LongPoller{Timeout: 30}, // Увеличиваем таймаут
// 	}

// 	// Добавляем обработку ошибки подключения
// 	b, err := telebot.NewBot(pref)
// 	if err != nil {
// 		log.Fatalf("Failed to create bot: %v\nPlease check your token and internet connection", err)
// 	}

// 	b.Handle("/start", func(c telebot.Context) error {
// 		return c.Send("Привет! Отправь мне ссылку на видео с YouTube, и я скачаю его для тебя.")
// 	})

// 	b.Handle(telebot.OnText, func(c telebot.Context) error {
// 		url := c.Text()
// 		userID := c.Chat().ID
// 		userState[userID] = url // Сохраняем URL для этого пользователя

// 		// Создаем кнопки
// 		selector := &telebot.ReplyMarkup{}
// 		btnMP4 := selector.Data("Скачать видео (MP4)", "mp4")
// 		btnMP3 := selector.Data("Скачать аудио (MP3)", "mp3")

// 		selector.Inline(
// 			selector.Row(btnMP4, btnMP3),
// 		)

// 		// Обработчики для кнопок
// 		b.Handle(&btnMP4, func(c telebot.Context) error {
// 			return handleFormatSelection(c, "mp4")
// 		})

// 		b.Handle(&btnMP3, func(c telebot.Context) error {
// 			return handleFormatSelection(c, "mp3")
// 		})

// 		return c.Send("Выберите формат:", selector)
// 	})

// 	log.Println("Бот запущен...")
// 	b.Start()
// }

// func handleFormatSelection(c telebot.Context, format string) error {
// 	userID := c.Chat().ID
// 	url, ok := userState[userID]
// 	if !ok {
// 		return c.Send("❌ Ссылка устарела. Отправьте новую.")
// 	}

// 	msg, _ := c.Bot().Send(c.Chat(), fmt.Sprintf("⏳ Начинаю загрузку %s...", format))

// 	filename, err := downloadMedia(url, format)
// 	if err != nil {
// 		c.Bot().Edit(msg, "❌ Ошибка: "+err.Error())
// 		return nil
// 	}
// 	defer os.Remove(filename)

// 	c.Bot().Edit(msg, fmt.Sprintf("✅ %s загружен! Отправляю...", strings.ToUpper(format)))

// 	var file interface{}
// 	if format == "mp4" {
// 		file = &telebot.Video{
// 			File:     telebot.FromDisk(filename),
// 			FileName: "video.mp4",
// 			MIME:     "video/mp4",
// 		}
// 	} else {
// 		file = &telebot.Audio{
// 			File:     telebot.FromDisk(filename),
// 			FileName: "audio.mp3",
// 			MIME:     "audio/mpeg",
// 		}
// 	}

// 	if _, err := c.Bot().Send(c.Chat(), file); err != nil {
// 		c.Bot().Edit(msg, "❌ Ошибка отправки файла")
// 		return err
// 	}

// 	c.Bot().Delete(msg)
// 	delete(userState, userID) // Очищаем состояние
// 	return nil
// }
//
