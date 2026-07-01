#!/usr/bin/env python3
"""
XinWiki MCP Server Package

A Model Context Protocol server that provides access to the XinWiki knowledge management API.
"""

__version__ = "1.0.0"
__author__ = "XinWiki Team"
__description__ = "XinWiki MCP Server - Model Context Protocol server for XinWiki API"

from .xinwiki_mcp_server import XinWikiClient, run

__all__ = ["XinWikiClient", "run"]
