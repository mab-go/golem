package main

import "github.com/mab-go/golem/internal/logging"

const (
	eventFatalCommand logging.Event = "main.fatal_command"
	eventStarting     logging.Event = "main.starting"
	eventShutdown     logging.Event = "main.shutdown"
	eventConnecting   logging.Event = "main.connecting"
	eventBotPosition  logging.Event = "main.bot_position"
	eventTestFail     logging.Event = "main.test_fail"
	eventTestOK       logging.Event = "main.test_ok"
	eventTestComplete logging.Event = "main.test_complete"
)
