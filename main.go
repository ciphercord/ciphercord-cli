package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	ccbot "github.com/ciphercord/gophercord/bot"
	ccmsg "github.com/ciphercord/gophercord/message"

	"github.com/eiannone/keyboard"
	"golang.org/x/term"
)

const (
	ps1 string = ":"
)

var (
	state *term.State
	wg    sync.WaitGroup

	key, room, name string = "MyPrivateKey", "CipherCord", "NoNickname"

	input     []byte
	cursorPos int = 0
)

func main() {
	flag.StringVar(&key, "key", key, "The encryption key")
	flag.StringVar(&room, "room", room, "The message space")
	flag.StringVar(&name, "name", name, "The nickname")

	flag.Usage = func() {
		fmt.Println("Usage: ciphercord-cli [options...]")
		fmt.Println(" -key  The encryption key (Default: MyPrivateKey)")
		fmt.Println(" -room The message space  (Default: CipherCord)")
		fmt.Println(" -name The nickname       (Default: NoNickname)")
	}

	flag.Parse()

	if err := keyboard.Open(); err != nil {
		fmt.Println(err)
		return
	}
	defer keyboard.Close()

	if err := ccbot.Init(); err != nil {
		fmt.Println(err)
		return
	}

	var err error
	state, err = term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer term.Restore(int(os.Stdin.Fd()), state)

	wg.Add(1)
	go chatBar()

	wg.Add(1)
	go receiving()

	wg.Wait()
}

func read() {
read:
	for {
		if cursorPos > len(input) {
			cursorPos = len(input)
		}
		prompt()

		r, key, err := keyboard.GetKey()
		if err != nil {
			clsl()
			println(err)
		}

		switch key {
		case '\r':
			break read
		case keyboard.KeyArrowDown, keyboard.KeyArrowUp:
		case keyboard.KeyArrowLeft:
			if cursorPos > 0 {
				cursorPos--
			}
		case keyboard.KeyArrowRight:
			if cursorPos < len(input) {
				cursorPos++
			}
		case keyboard.KeyBackspace, keyboard.KeyBackspace2:
			if len(input) > 0 {
				var newInput []byte
				for i, r := range input {
					if i != cursorPos-1 {
						newInput = append(newInput, r)
					}
				}
				input = newInput
				cursorPos--
			}
		case keyboard.KeyDelete:
			if len(input) > 0 {
				var newInput []byte
				for i, r := range input {
					if i != cursorPos {
						newInput = append(newInput, r)
					}
				}
				input = newInput
			}
		case keyboard.KeyCtrlC:
			exit()
		default:
			if key == keyboard.KeySpace || r == 0 {
				r = ' '
			}
			input = append(input, 0)
			copy(input[cursorPos+1:], input[cursorPos:])
			input[cursorPos] = byte(r)
			cursorPos++
		}
	}
}

func prompt() {
	clsl()
	fmt.Print(ps1)
	fmt.Print(string(input))
	move(cursorPos + len(ps1) + 1)
}

func chatBar() {
	defer wg.Done()

	for {
		read()

		if len(input) == 0 {
			continue
		}

		s := string(input)
		input = []byte{}

		if strings.HasPrefix(s, "/") {
			command(s)
			continue
		}

		var umsg ccmsg.UnencryptedMessage
		umsg.Key = key
		umsg.Room = room
		umsg.Content = s
		umsg.Author = name

		data, err := ccmsg.Package(umsg)
		if err != nil {
			clsl()
			println(err)
			prompt()
			continue
		}

		if err := ccbot.Send(data); err != nil {
			clsl()
			println(err)
			prompt()
			continue
		}

		prompt()
	}
}

func command(s string) {
	if strings.HasPrefix(s, "/help") {
		clsl()
		println("/help        Show this menu")
		println("/name <name> Set your nickname")
		println("/room <room> Move rooms")
		println("/key <key>   Change encryption key")
	} else if str, found := strings.CutPrefix(s, "/name "); found {
		name = str
	} else if str, found := strings.CutPrefix(s, "/room "); found {
		room = str
	} else if str, found := strings.CutPrefix(s, "/key "); found {
		key = str
	} else if strings.HasPrefix(s, "/exit") {
		exit()
	} else {
		clsl()
		println("Unknown command.")
	}
}

func receiving() {
	defer wg.Done()

	for {
		data := <-ccbot.Messages

		emsg, err := ccmsg.Decode(data)
		if err != nil {
			clsl()
			println(err)
			prompt()
			continue
		}
		if emsg.Room != room {
			continue
		}

		umsg, err := ccmsg.DecryptMessage(emsg, key)
		if err == ccmsg.ErrUnmatched {
			continue
		} else if err != nil {
			clsl()
			println(err)
			prompt()
			continue
		}

		clsl()
		fmt.Printf("%s: %s\r\n", umsg.Author, umsg.Content)
		prompt()
	}
}

func exit() {
	wg.Done()
	wg.Done()
}

func clsl() { // clear line
	fmt.Print("\033[2K\r")
}

func println(a ...any) {
	fmt.Print(a...)
	fmt.Print("\r\n")
}

func move(i int) {
	fmt.Printf("\033[%dG", i)
}
