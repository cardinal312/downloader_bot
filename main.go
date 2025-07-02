package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gopkg.in/telebot.v3"
	"gopkg.in/yaml.v3"
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

var cfg Config

func init() {
	loadConfig()
	checkDependencies()
}

func loadConfig() {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Error parsing config: %v", err)
	}

	if cfg.Telegram.Token == "" {
		log.Fatal("Telegram token not configured")
	}

	if cfg.Download.OutputDir == "" {
		cfg.Download.OutputDir = "downloads"
	}

	if cfg.Download.MaxSizeMB == 0 {
		cfg.Download.MaxSizeMB = 50
	}

	if err := os.MkdirAll(cfg.Download.OutputDir, 0755); err != nil {
		log.Fatalf("Error creating downloads dir: %v", err)
	}
}

func checkDependencies() {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		log.Fatal("yt-dlp not found. Please install it first.")
	}

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		log.Fatal("ffmpeg not found. Please install it first.")
	}
}

func downloadVideo(url string) (string, error) {
	filename := filepath.Join(cfg.Download.OutputDir, fmt.Sprintf("%d.mp4", time.Now().Unix()))

	args := []string{
		"--no-check-certificate",
		"--force-ipv4",
		"--geo-bypass",
		"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best",
		"--merge-output-format", "mp4",
		"--recode-video", "mp4",
		"--postprocessor-args", "ffmpeg:-c:v libx264 -preset fast -crf 22 -c:a aac -b:a 128k -movflags +faststart",
		"-o", filename,
		url,
	}

	if cfg.Download.CookiesFile != "" {
		args = append([]string{"--cookies", cfg.Download.CookiesFile}, args...)
	}

	cmd := exec.Command("yt-dlp", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("download failed: %v\n%s", err, string(output))
	}

	return filename, nil
}

func main() {
	pref := telebot.Settings{
		Token:  cfg.Telegram.Token,
		Poller: &telebot.LongPoller{Timeout: 10},
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatal(err)
	}

	b.Handle("/start", func(c telebot.Context) error {
		return c.Send("Привет! Отправь мне ссылку на видео с YouTube, и я скачаю его для тебя.")
	})

	b.Handle(telebot.OnText, func(c telebot.Context) error {
		url := c.Text()
		msg, _ := b.Send(c.Chat(), "⏳ Начинаю загрузку...")

		filename, err := downloadVideo(url)
		if err != nil {
			b.Edit(msg, "❌ Ошибка: "+err.Error())
			return nil
		}
		defer os.Remove(filename)

		b.Edit(msg, "✅ Видео загружено! Отправляю...")

		video := &telebot.Video{
			File:     telebot.FromDisk(filename),
			FileName: "video.mp4",
			MIME:     "video/mp4",
		}

		if _, err := b.Send(c.Chat(), video); err != nil {
			b.Edit(msg, "❌ Ошибка отправки видео")
			return err
		}

		b.Delete(msg)
		return nil
	})

	log.Println("Бот запущен...")
	b.Start()
}