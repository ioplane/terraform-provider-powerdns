"""The repository's own checks.

Each module is one check, runnable as `python -m scripts.checks.<name>` and
importable so its decisions can be tested without a subprocess. They were
shell scripts until they grew conditionals nobody could test.
"""
