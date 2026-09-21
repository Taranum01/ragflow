#
#  Copyright 2026 The InfiniFlow Authors. All Rights Reserved.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
import ast
from pathlib import Path

import pytest

pytestmark = pytest.mark.p2


def test_retrieve_kb_calls_kg_retriever_once():
    source = Path("agent/tools/retrieval.py").read_text(encoding="utf-8")
    tree = ast.parse(source)
    retrieve_kb = next(
        node
        for node in ast.walk(tree)
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) and node.name == "_retrieve_kb"
    )
    calls = [
        node
        for node in ast.walk(retrieve_kb)
        if isinstance(node, ast.Call)
        and isinstance(node.func, ast.Attribute)
        and node.func.attr == "retrieval"
        and isinstance(node.func.value, ast.Attribute)
        and node.func.value.attr == "kg_retriever"
    ]
    assert len(calls) == 1
