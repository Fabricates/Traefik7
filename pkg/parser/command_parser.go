package parser

import (
	"fmt"
	"strings"
)

// F5Command represents a parsed F5 command
type F5Command struct {
	Action     string            // add, bind, set, etc.
	ObjectType string            // server, lb vserver, serviceGroup, etc.
	Name       string            // object name
	Arguments  []string          // positional arguments
	Parameters map[string]string // named parameters (-param value)
}

// CommandParser parses F5 commands using proper syntax analysis
type CommandParser struct {
	tokens  []Token
	pos     int
	current Token
}

// NewCommandParser creates a new command parser
func NewCommandParser(tokens []Token) *CommandParser {
	p := &CommandParser{
		tokens: tokens,
		pos:    0,
	}
	p.readToken()
	return p
}

// readToken advances to the next token
func (p *CommandParser) readToken() {
	if p.pos < len(p.tokens) {
		p.current = p.tokens[p.pos]
		p.pos++
	} else {
		p.current = Token{Type: TokenEOF}
	}
}

// peekToken returns the next token without advancing
func (p *CommandParser) peekToken() Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Type: TokenEOF}
}

// expectToken expects a specific token type and advances
func (p *CommandParser) expectToken(expectedType TokenType) (Token, error) {
	if p.current.Type != expectedType {
		return Token{}, fmt.Errorf("expected %v, got %v at line %d column %d",
			expectedType, p.current.Type, p.current.Line, p.current.Column)
	}
	token := p.current
	p.readToken()
	return token, nil
}

// parseAction parses the command action (add, bind, set, etc.)
func (p *CommandParser) parseAction() (string, error) {
	switch p.current.Type {
	case TokenAdd:
		token := p.current
		p.readToken()
		return token.Value, nil
	case TokenBind:
		token := p.current
		p.readToken()
		return token.Value, nil
	case TokenSet:
		token := p.current
		p.readToken()
		return token.Value, nil
	case TokenUnbind:
		token := p.current
		p.readToken()
		return token.Value, nil
	case TokenRemove:
		token := p.current
		p.readToken()
		return token.Value, nil
	case TokenLink:
		token := p.current
		p.readToken()
		return token.Value, nil
	default:
		return "", fmt.Errorf("expected action (add, bind, set, etc.), got %v at line %d",
			p.current.Type, p.current.Line)
	}
}

// parseObjectType parses the object type (server, lb vserver, serviceGroup, etc.)
func (p *CommandParser) parseObjectType() (string, error) {
	var parts []string

	// Handle compound object types by collecting consecutive object type tokens or identifiers
	for {
		// Check if current token is a recognized object type or identifier
		isObjectTypeToken := false

		switch p.current.Type {
		case TokenServer, TokenLB, TokenVServer, TokenServiceGroup, TokenMonitor,
			TokenAudit, TokenAuthentication, TokenCache, TokenCS, TokenDNS,
			TokenRoute, TokenResponder, TokenRewrite, TokenPolicy, TokenPolicyLabel,
			TokenAction, TokenContentGroup, TokenNameServer, TokenAddRec, TokenNSRec,
			TokenSSL, TokenSystem, TokenTM, TokenTunnel, TokenAAA, TokenAppflow,
			TokenCMP, TokenNS, TokenSubscriber, TokenVPN, TokenDB,
			TokenNslogAction, TokenSyslogAction, TokenSyslogPolicy,
			TokenNoAuthAction, TokenTacacsAction, TokenTacacsPolicy,
			TokenCertKey, TokenCmdPolicy, TokenNslogGlobal, TokenSyslogGlobal,
			TokenGlobal, TokenPatset, TokenService, TokenUser, TokenGroup,
			TokenParam, TokenDiameter, TokenEncryptionParams, TokenHttpParam,
			TokenHttpProfile, TokenRpcNode, TokenTcpbufParam, TokenGxInterface:
			parts = append(parts, p.current.Value)
			p.readToken()
			isObjectTypeToken = true
		case TokenIdentifier:
			// Allow identifiers as part of object types (like "nslogAction", "syslogAction", etc.)
			parts = append(parts, p.current.Value)
			p.readToken()
			isObjectTypeToken = true
		}

		if !isObjectTypeToken {
			break
		}

		// Check if we should continue parsing compound object types
		next := p.current
		if next.Type == TokenEOF || next.Type == TokenParameterFlag {
			break
		}

		// Continue if it's another object type component or identifier
		isNextObjectType := next.Type == TokenServer || next.Type == TokenLB ||
			next.Type == TokenVServer || next.Type == TokenServiceGroup ||
			next.Type == TokenMonitor ||
			next.Type == TokenAudit || next.Type == TokenAuthentication ||
			next.Type == TokenCache || next.Type == TokenCS || next.Type == TokenDNS ||
			next.Type == TokenRoute || next.Type == TokenResponder || next.Type == TokenRewrite ||
			next.Type == TokenPolicy || next.Type == TokenAction ||
			next.Type == TokenContentGroup || next.Type == TokenNameServer ||
			next.Type == TokenAddRec || next.Type == TokenNSRec || next.Type == TokenSSL ||
			next.Type == TokenSystem || next.Type == TokenTM || next.Type == TokenTunnel ||
			next.Type == TokenAAA || next.Type == TokenAppflow || next.Type == TokenCMP ||
			next.Type == TokenNS || next.Type == TokenSubscriber || next.Type == TokenVPN ||
			next.Type == TokenDB || next.Type == TokenNslogAction || next.Type == TokenSyslogAction ||
			next.Type == TokenSyslogPolicy || next.Type == TokenNoAuthAction ||
			next.Type == TokenTacacsAction || next.Type == TokenTacacsPolicy ||
			next.Type == TokenCertKey || next.Type == TokenCmdPolicy

		if !isNextObjectType {
			break
		}
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("expected object type (server, lb vserver, serviceGroup, etc.), got %v (%s) at line %d",
			p.current.Type, p.current.Value, p.current.Line)
	}

	return strings.Join(parts, " "), nil
} // parseObjectName parses the object name (can be quoted or unquoted)
func (p *CommandParser) parseObjectName() (string, error) {
	switch p.current.Type {
	case TokenString:
		token := p.current
		p.readToken()
		return token.Value, nil
	case TokenIdentifier:
		token := p.current
		p.readToken()
		return token.Value, nil
	case TokenNumber:
		token := p.current
		p.readToken()
		return token.Value, nil
	case TokenIP:
		token := p.current
		p.readToken()
		return token.Value, nil
	default:
		// Try to accept any token that has a meaningful value as an object name
		if p.current.Type != TokenEOF && p.current.Type != TokenParameterFlag && p.current.Value != "" {
			token := p.current
			p.readToken()
			return token.Value, nil
		}
		return "", fmt.Errorf("expected object name (string, identifier, number, or IP), got %v (%s) at line %d",
			p.current.Type, p.current.Value, p.current.Line)
	}
}

// parseArguments parses positional arguments until we hit parameters or EOF
func (p *CommandParser) parseArguments() []string {
	var args []string

	for p.current.Type != TokenEOF && p.current.Type != TokenParameterFlag {
		// Accept any token with a meaningful value as an argument
		if p.current.Value != "" {
			args = append(args, p.current.Value)
			p.readToken()
		} else {
			// Unknown token, stop parsing arguments
			break
		}
	}

	return args
}

// parseParameters parses named parameters (-param value pairs)
func (p *CommandParser) parseParameters() map[string]string {
	params := make(map[string]string)

	for p.current.Type == TokenParameterFlag {
		paramName := p.current.Value
		p.readToken()

		// Get parameter value
		var paramValue string
		switch p.current.Type {
		case TokenString, TokenIdentifier, TokenNumber, TokenIP:
			paramValue = p.current.Value
			p.readToken()
		default:
			// Parameter without value, use empty string
			paramValue = ""
		}

		params[paramName] = paramValue
	}

	return params
}

// ParseCommand parses a complete F5 command
func (p *CommandParser) ParseCommand() (*F5Command, error) {
	if p.current.Type == TokenEOF {
		return nil, fmt.Errorf("empty command")
	}

	// Parse action
	action, err := p.parseAction()
	if err != nil {
		return nil, err
	}

	// Parse object type
	objectType, err := p.parseObjectType()
	if err != nil {
		return nil, err
	}

	// Parse object name
	name, err := p.parseObjectName()
	if err != nil {
		return nil, err
	}

	// Parse positional arguments
	arguments := p.parseArguments()

	// Parse named parameters
	parameters := p.parseParameters()

	return &F5Command{
		Action:     action,
		ObjectType: objectType,
		Name:       name,
		Arguments:  arguments,
		Parameters: parameters,
	}, nil
}

// ParseF5Command is a convenience function to parse a command string
func ParseF5Command(commandLine string) (*F5Command, error) {
	// Skip empty lines and comments
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" || strings.HasPrefix(commandLine, "#") {
		return nil, nil
	}

	// Tokenize the command
	tokens := TokenizeCommand(commandLine)

	// Check for tokenization errors
	for _, token := range tokens {
		if token.Type == TokenError {
			return nil, fmt.Errorf("tokenization error: %s", token.Value)
		}
	}

	// Parse the command
	parser := NewCommandParser(tokens)
	return parser.ParseCommand()
}
