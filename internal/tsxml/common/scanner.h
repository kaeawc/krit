#pragma once

#include "tree_sitter/parser.h"

enum TokenType {
    PI_TARGET,
    PI_CONTENT,
    COMMENT,

#ifdef TS_XML
    CHAR_DATA,
    CDATA,
    XML_MODEL,
    XML_STYLESHEET,
    START_TAG_NAME,
    END_TAG_NAME,
    ERRONEOUS_END_NAME,
    SELF_CLOSING_TAG_DELIMITER,
#endif
};

/// Advance the lexer if the next token matches the given character
#define advance_if_eq(lexer, chr) \
    if (!lexer->eof(lexer) && (lexer)->lookahead == (chr)) advance((lexer)); else return false

#ifdef _WIN32
#undef max
#undef min
#endif

/// Advance the lexer to the next token
static inline void advance(TSLexer *lexer) { lexer->advance(lexer, false); }

/// Check if the character is valid in a name
/// Follows https://www.w3.org/TR/xml11/#NT-Name
static inline bool is_in_range(int32_t chr, int32_t lo, int32_t hi) {
    return chr >= lo && chr <= hi;
}

static inline bool is_valid_name_start_char(int32_t chr) {
    if (chr >= 0x80) {
        return is_in_range(chr, 0xC0, 0xD6) ||
            is_in_range(chr, 0xD8, 0xF6) ||
            is_in_range(chr, 0xF8, 0x2FF) ||
            is_in_range(chr, 0x370, 0x37D) ||
            is_in_range(chr, 0x37F, 0x1FFF) ||
            is_in_range(chr, 0x200C, 0x200D) ||
            is_in_range(chr, 0x2070, 0x218F) ||
            is_in_range(chr, 0x2C00, 0x2FEF) ||
            is_in_range(chr, 0x3001, 0xD7FF) ||
            is_in_range(chr, 0xF900, 0xFDCF) ||
            is_in_range(chr, 0xFDF0, 0xFFFD) ||
            is_in_range(chr, 0x10000, 0xEFFFF);
    }

    return (chr >= 'A' && chr <= 'Z') ||
        (chr >= 'a' && chr <= 'z') ||
        chr == '_' || chr == ':';
}

static inline bool is_valid_name_char(int32_t chr) {
    return is_valid_name_start_char(chr) ||
        chr == '-' ||
        chr == '.' ||
        (chr >= '0' && chr <= '9') ||
        chr == 0xB7 ||
        is_in_range(chr, 0x300, 0x36F) ||
        is_in_range(chr, 0x203F, 0x2040);
}

/// Check if the lexer matches the given word
static inline bool check_word(TSLexer *lexer, const char *const word, unsigned length) {
    for (unsigned j = 0; j < length; ++j) {
        advance_if_eq(lexer, word[j]);
    }
    return true;
}

/// Scan for the target of a PI node
static bool scan_pi_target(TSLexer *lexer, const bool *valid_symbols) {
    bool advanced_once = false, found_x_first = false;
#ifndef TS_XML
    (void)valid_symbols;
#endif

    if (is_valid_name_start_char(lexer->lookahead)) {
        if (lexer->lookahead == 'x' || lexer->lookahead == 'X') {
            found_x_first = true;
            lexer->mark_end(lexer);
        }
        advanced_once = true;
        advance(lexer);
    }

    if (advanced_once) {
        while (is_valid_name_char(lexer->lookahead)) {
            if (found_x_first && (lexer->lookahead == 'm' || lexer->lookahead == 'M')) {
                advance(lexer);
                if (lexer->lookahead == 'l' || lexer->lookahead == 'L') {
                    advance(lexer);
                    if (is_valid_name_char(lexer->lookahead)) {
#ifdef TS_XML
                        found_x_first = false;
                        bool last_char_hyphen = lexer->lookahead == '-';
                        advance(lexer);
                        if (last_char_hyphen) {
                            if (valid_symbols[XML_MODEL] && check_word(lexer, "model", 5))
                                return false;
                            if (valid_symbols[XML_STYLESHEET] && check_word(lexer, "stylesheet", 10))
                                return false;
                        }
#endif
                    } else {
                        return false;
                    }
                }
            }

            found_x_first = false;
            advance(lexer);
        }

        lexer->mark_end(lexer);
        lexer->result_symbol = PI_TARGET;
        return true;
    }

    return false;
}

/// Scan for the content of a PI node
static bool scan_pi_content(TSLexer *lexer) {
    while (!lexer->eof(lexer) && lexer->lookahead != '\n' && lexer->lookahead != '?')
        advance(lexer);

    if (lexer->lookahead != '?')
        return false;

    lexer->mark_end(lexer);
    advance(lexer);

    if (lexer->lookahead == '>') {
        advance(lexer);
        while (lexer->lookahead == ' ')
            advance(lexer);
        advance_if_eq(lexer, '\n');
        lexer->result_symbol = PI_CONTENT;
        return true;
    }

    return false;
}

/// Scan for a Comment node
static bool scan_comment(TSLexer *lexer) {
    advance_if_eq(lexer, '-');
    advance_if_eq(lexer, '-');

    while (!lexer->eof(lexer)) {
        if (lexer->lookahead == '-') {
            advance(lexer);
            if (lexer->lookahead == '-') {
                advance(lexer);
                break;
            }
        } else {
            advance(lexer);
        }
    }

    if (lexer->lookahead == '>') {
        advance(lexer);
        lexer->mark_end(lexer);
        lexer->result_symbol = COMMENT;
        return true;
    }

    return false;
}
