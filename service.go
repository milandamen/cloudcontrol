package main

import (
	"fmt"
	"os"

	"github.com/kardianos/service"
	"github.com/pkg/errors"
)

func main() {
	var webadmin bool
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "--help" {
			fmt.Println("Help commands:")
			fmt.Println()
			fmt.Println("--create-config: create config file in current working directory")
			fmt.Println("--add-remote <host>: add remote to the config file")
			fmt.Println("--webadmin: allow users to connect to a web admin interface on port 2001")
			return
		}

		if arg == "--create-config" {
			if err := writeConfig(Config{}); err != nil {
				fmt.Println(errors.Wrap(err, "cannot create config in current working directory"))
				os.Exit(1)
				return
			}

			if err := writeNewSelfKey(); err != nil {
				fmt.Println(errors.Wrap(err, "cannot write new key"))
				os.Exit(1)
				return
			}

			return
		}

		if arg == "--webadmin" {
			webadmin = true
		}

		if len(os.Args) > 2 {
			if arg == "--add-remote" {
				if err := addRemote(os.Args[2]); err != nil {
					fmt.Println(errors.Wrap(err, "cannot add remote"))
					os.Exit(1)
					return
				}

				return
			}
		}
	}

	svcConfig := &service.Config{
		Name:        "cloudcontrol",
		DisplayName: "Cloud Control",
		Description: "Control the home cloud",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var logger service.Logger
	logger, err = s.Logger(nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	prg.Logger = logger
	prg.Webadmin = webadmin

	var c Config
	c, err = loadConfig()
	if err != nil {
		_ = prg.Logger.Error(errors.Wrap(err, "cannot load config"))
		if cause := errors.Cause(err); os.IsNotExist(cause) {
			_ = prg.Logger.Error("Run program with --create-config to create config in current working directory")
		}

		os.Exit(1)
		return
	}

	prg.Config = c
	if err = prg.validateConfig(); err != nil {
		_ = prg.Logger.Error(errors.Wrap(err, "invalid config"))
		os.Exit(1)
		return
	}

	err = s.Run()
	if err != nil {
		_ = logger.Error(err)
	}
}
