"""glyphmesh asset service: from-scratch PNG encoder + procedural generator."""

from .png import encode_png
from .generator import generate_asset

__all__ = ["encode_png", "generate_asset"]
__version__ = "0.1.0"
