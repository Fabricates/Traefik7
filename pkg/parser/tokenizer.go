package parser

import (
	"fmt"
	"strings"
	"unicode"
)

// TokenType represents the type of token
type TokenType int

const (
	// Command tokens
	TokenAdd TokenType = iota
	TokenBind
	TokenSet
	TokenUnbind
	TokenRemove
	TokenLink

	// Object types - Main categories
	TokenServer
	TokenLB
	TokenVServer
	TokenServiceGroup
	TokenMonitor
	TokenAudit
	TokenAuthentication
	TokenCache
	TokenCS
	TokenDNS
	TokenRoute
	TokenResponder
	TokenRewrite
	TokenPolicy
	TokenPolicyLabel
	TokenAction
	TokenContentGroup
	TokenNameServer
	TokenAddRec
	TokenNSRec
	TokenSSL
	TokenSystem
	TokenTM
	TokenTunnel
	TokenAAA
	TokenAppflow
	TokenCMP
	TokenNS
	TokenSubscriber
	TokenVPN
	TokenDB

	// Sub-types for compound object types
	TokenNslogAction
	TokenSyslogAction
	TokenSyslogPolicy
	TokenNoAuthAction
	TokenTacacsAction
	TokenTacacsPolicy
	TokenCertKey
	TokenCmdPolicy
	TokenNslogGlobal
	TokenSyslogGlobal
	TokenGlobal
	TokenPatset
	TokenService
	TokenUser
	TokenGroup
	TokenParam
	TokenDiameter
	TokenEncryptionParams
	TokenHttpParam
	TokenHttpProfile
	TokenRpcNode
	TokenTcpbufParam
	TokenGxInterface

	// Identifiers and values
	TokenIdentifier
	TokenString
	TokenNumber
	TokenIP

	// Parameters
	TokenParameterFlag // Parameters that start with -

	// Special tokens
	TokenEOF
	TokenError
)

// Token represents a lexical token
type Token struct {
	Type   TokenType
	Value  string
	Line   int
	Column int
}

// Tokenizer handles lexical analysis of Citrix commands
type Tokenizer struct {
	input   string
	pos     int
	line    int
	column  int
	current rune
}

// NewTokenizer creates a new tokenizer
func NewTokenizer(input string) *Tokenizer {
	t := &Tokenizer{
		input:  input,
		pos:    0,
		line:   1,
		column: 1,
	}
	t.readChar()
	return t
}

// readChar reads the next character
func (t *Tokenizer) readChar() {
	if t.pos >= len(t.input) {
		t.current = 0 // EOF
	} else {
		t.current = rune(t.input[t.pos])
	}
	t.pos++
	if t.current == '\n' {
		t.line++
		t.column = 1
	} else {
		t.column++
	}
}

// skipWhitespace skips whitespace characters
func (t *Tokenizer) skipWhitespace() {
	for unicode.IsSpace(t.current) && t.current != '\n' {
		t.readChar()
	}
}

// readString reads a quoted string
func (t *Tokenizer) readString() string {
	var result strings.Builder
	quote := t.current
	t.readChar() // skip opening quote

	for t.current != quote && t.current != 0 {
		if t.current == '\\' {
			t.readChar()
			if t.current != 0 {
				result.WriteRune(t.current)
				t.readChar()
			}
		} else {
			result.WriteRune(t.current)
			t.readChar()
		}
	}

	if t.current == quote {
		t.readChar() // skip closing quote
	}

	return result.String()
}

// readIdentifier reads an identifier or keyword
func (t *Tokenizer) readIdentifier() string {
	var result strings.Builder

	for unicode.IsLetter(t.current) || unicode.IsDigit(t.current) ||
		t.current == '_' || t.current == '-' || t.current == ':' || t.current == '.' {
		result.WriteRune(t.current)
		t.readChar()
	}

	return result.String()
}

// readParameter reads a parameter starting with -
func (t *Tokenizer) readParameter() string {
	var result strings.Builder
	result.WriteRune(t.current) // include the -
	t.readChar()

	for unicode.IsLetter(t.current) || unicode.IsDigit(t.current) || t.current == '_' {
		result.WriteRune(t.current)
		t.readChar()
	}

	return result.String()
}

// isIPAddress checks if a string looks like an IP address
func (t *Tokenizer) isIPAddress(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 || len(part) > 3 {
			return false
		}
		for _, r := range part {
			if !unicode.IsDigit(r) {
				return false
			}
		}
	}
	return true
}

// getKeywordType returns the token type for keywords
func (t *Tokenizer) getKeywordType(value string) TokenType {
	switch strings.ToLower(value) {
	// Commands
	case "add":
		return TokenAdd
	case "bind":
		return TokenBind
	case "set":
		return TokenSet
	case "unbind":
		return TokenUnbind
	case "remove":
		return TokenRemove
	case "link":
		return TokenLink

	// Main object types
	case "server":
		return TokenServer
	case "lb":
		return TokenLB
	case "vserver":
		return TokenVServer
	case "servicegroup":
		return TokenServiceGroup
	case "monitor":
		return TokenMonitor
	case "audit":
		return TokenAudit
	case "authentication":
		return TokenAuthentication
	case "cache":
		return TokenCache
	case "cs":
		return TokenCS
	case "dns":
		return TokenDNS
	case "route":
		return TokenRoute
	case "responder":
		return TokenResponder
	case "rewrite":
		return TokenRewrite
	case "policy":
		return TokenPolicy
	case "policylabel":
		return TokenPolicyLabel
	case "action":
		return TokenAction
	case "contentgroup":
		return TokenContentGroup
	case "nameserver":
		return TokenNameServer
	case "addrec":
		return TokenAddRec
	case "nsrec":
		return TokenNSRec
	case "ssl":
		return TokenSSL
	case "system":
		return TokenSystem
	case "tm":
		return TokenTM
	case "tunnel":
		return TokenTunnel
	case "aaa":
		return TokenAAA
	case "appflow":
		return TokenAppflow
	case "cmp":
		return TokenCMP
	case "ns":
		return TokenNS
	case "subscriber":
		return TokenSubscriber
	case "vpn":
		return TokenVPN
	case "db":
		return TokenDB

	// Sub-types for compound objects
	case "nslogaction":
		return TokenNslogAction
	case "syslogaction":
		return TokenSyslogAction
	case "syslogpolicy":
		return TokenSyslogPolicy
	case "noauthaction":
		return TokenNoAuthAction
	case "tacacsaction":
		return TokenTacacsAction
	case "tacacspolicy":
		return TokenTacacsPolicy
	case "certkey":
		return TokenCertKey
	case "cmdpolicy":
		return TokenCmdPolicy
	case "nslogglobal":
		return TokenNslogGlobal
	case "syslogglobal":
		return TokenSyslogGlobal
	case "global":
		return TokenGlobal
	case "patset":
		return TokenPatset
	case "service":
		return TokenService
	case "user":
		return TokenUser
	case "group":
		return TokenGroup
	case "parameter":
		return TokenParam
	case "param":
		return TokenParam
	case "diameter":
		return TokenDiameter
	case "encryptionparams":
		return TokenEncryptionParams
	case "httpparam":
		return TokenHttpParam
	case "httpprofile":
		return TokenHttpProfile
	case "rpcnode":
		return TokenRpcNode
	case "tcpbufparam":
		return TokenTcpbufParam
	case "gxinterface":
		return TokenGxInterface

	default:
		if t.isIPAddress(value) {
			return TokenIP
		}
		// Check if it's a number
		allDigits := true
		for _, r := range value {
			if !unicode.IsDigit(r) {
				allDigits = false
				break
			}
		}
		if allDigits {
			return TokenNumber
		}
		return TokenIdentifier
	}
}

// NextToken returns the next token
func (t *Tokenizer) NextToken() Token {
	t.skipWhitespace()

	token := Token{
		Line:   t.line,
		Column: t.column,
	}

	switch t.current {
	case 0:
		token.Type = TokenEOF
		token.Value = ""
	case '\n', '\r':
		// Skip newlines and return next token
		t.readChar()
		return t.NextToken()
	case '"', '\'':
		token.Type = TokenString
		token.Value = t.readString()
	case '-':
		token.Type = TokenParameterFlag
		token.Value = t.readParameter()
	case '.':
		// Handle standalone dot (like DNS root)
		token.Type = TokenIdentifier
		token.Value = "."
		t.readChar()
	default:
		if unicode.IsLetter(t.current) || unicode.IsDigit(t.current) || t.current == '_' {
			value := t.readIdentifier()
			token.Type = t.getKeywordType(value)
			token.Value = value
		} else {
			token.Type = TokenError
			token.Value = fmt.Sprintf("unexpected character: %c", t.current)
			t.readChar()
		}
	}

	return token
}

// TokenizeCommand tokenizes a complete command line
func TokenizeCommand(command string) []Token {
	tokenizer := NewTokenizer(command)
	var tokens []Token

	for {
		token := tokenizer.NextToken()
		tokens = append(tokens, token)
		if token.Type == TokenEOF || token.Type == TokenError {
			break
		}
	}

	return tokens
}
