import io
import logging

from real_estate_etl.logging_config import _HttpxRequestDebugFilter, _PipelineFormatter


def test_httpx_info_requests_are_downgraded_to_debug():
    record = logging.LogRecord(
        "httpx", logging.INFO, __file__, 1, "HTTP Request: GET signed-url", (), None
    )

    assert _HttpxRequestDebugFilter().filter(record)
    assert record.levelno == logging.DEBUG
    assert record.levelname == "DEBUG"


def test_pipeline_info_records_keep_their_level():
    record = logging.LogRecord(
        "real_estate_etl.pipeline", logging.INFO, __file__, 1, "Imported rows", (), None
    )

    assert _HttpxRequestDebugFilter().filter(record)
    assert record.levelno == logging.INFO


def test_downgraded_httpx_request_is_filtered_by_info_handler():
    output = io.StringIO()
    handler = logging.StreamHandler(output)
    handler.setLevel(logging.INFO)
    logger = logging.Logger("httpx", level=logging.DEBUG)
    logger.addFilter(_HttpxRequestDebugFilter())
    logger.addHandler(handler)

    logger.info("HTTP Request: GET signed-url")

    assert output.getvalue() == ""


def test_plan_block_is_rendered_without_partial_log_prefix():
    record = logging.LogRecord(
        "real_estate_etl.operations",
        logging.INFO,
        __file__,
        1,
        "==== PLAN ====\nfirst stage",
        (),
        None,
    )
    record.plain_block = True
    formatter = _PipelineFormatter("%(levelname)s %(name)s — %(message)s")

    assert formatter.format(record) == "==== PLAN ====\nfirst stage"
