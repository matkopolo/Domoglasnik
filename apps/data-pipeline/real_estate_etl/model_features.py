"""Feature preprocessing and estimator construction for valuation models."""

from sklearn.compose import ColumnTransformer
from sklearn.impute import SimpleImputer
from sklearn.linear_model import Ridge
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import OneHotEncoder, StandardScaler

MODEL_FEATURES = [
    "log_area",
    "construction_year",
    "contract_year",
    "contract_month",
    "centroid_e",
    "centroid_n",
    "property_group",
    "municipality_id",
]


def preprocessor(
    numeric_columns: list[int], categorical_columns: list[int], *, dense: bool = False
) -> ColumnTransformer:
    return ColumnTransformer(
        [
            (
                "numeric",
                Pipeline(
                    [
                        ("impute", SimpleImputer(strategy="median", keep_empty_features=True)),
                        ("scale", StandardScaler()),
                    ]
                ),
                numeric_columns,
            ),
            (
                "categorical",
                OneHotEncoder(
                    handle_unknown="ignore",
                    min_frequency=10,
                    sparse_output=not dense,
                ),
                categorical_columns,
            ),
        ],
        sparse_threshold=0 if dense else 0.3,
    )


def hedonic_pipeline() -> Pipeline:
    return Pipeline(
        [
            ("features", preprocessor(list(range(6)), [6, 7])),
            ("regression", Ridge(alpha=2.0)),
        ]
    )
