SELECT  
identificatienummer as "Patient.id",
    metingid as "resource_id",
    metingid AS id,
    '' AS parent_id,
    'Observation' AS fhir_path,
    'final' AS "status",
    'text' AS "category[0].text",
    'text1' AS "category[1].text",
    'http://terminology.hl7.org/CodeSystem/observation-category' AS "category[0].coding[0].system",
    'laboratory' AS "category[0].coding[0].code",
    'Laboratory' AS "category[0].coding[0].display",
    'http://terminology.hl7.org/CodeSystem/observation-category' AS "category[1].coding[0].system",
    'laboratory' AS "category[1].coding[0].code",
    'Laboratory' AS "category[1].coding[0].display",
    'http://terminology.hl7.org/CodeSystem/observation-category' AS "category[1].coding[2].system",
    'nogiets' AS "category[1].coding[2].code",
    'Laboratory' AS "category[1].coding[2].display",

    -- -- COALESCE(metingnaamcodesysteem, 'unknown') AS "code.coding.code.system",
    -- -- metingnaamcode AS "code.coding.code.code",
    -- -- metingnaamomschrijving AS "code.coding.code.display",
    'Patient/' || identificatienummer AS "subject.reference",
    metingdatumtijd AS "effective.dateTime",
    CASE
        WHEN uitslagwaarde ~ '^[-+]?[0-9]*\.?[0-9]+$' THEN 'valueQuantity'
        WHEN uitslagcode IS NOT NULL THEN 'valueCodeableConcept'
        ELSE 'valueString'
    END AS "value.type",
    CASE
        WHEN uitslagwaarde ~ '^[-+]?[0-9]*\.?[0-9]+$' THEN uitslagwaarde::numeric
        ELSE NULL
    END AS "value.valueQuantity.value",
    CASE
        WHEN uitslagwaarde ~ '^[-+]?[0-9]*\.?[0-9]+$' THEN uitslagwaardeeenheid
        ELSE NULL
    END AS "value.valueQuantity.unit",
    CASE
        WHEN uitslagwaarde ~ '^[-+]?[0-9]*\.?[0-9]+$' THEN 'http://unitsofmeasure.org'
        ELSE NULL
    END AS "value.valueQuantity.system",
    CASE
        WHEN uitslagwaarde ~ '^[-+]?[0-9]*\.?[0-9]+$' THEN uitslagwaardeeenheid
        ELSE NULL
    END AS "value.valueQuantity.code",
    CASE
        WHEN uitslagcode IS NOT NULL THEN COALESCE(uitslagcodesysteem, 'unknown')
        ELSE NULL
    END AS "value.valueCodeableConcept.coding.system",
    uitslagcode AS "value.valueCodeableConcept.coding.code",
    uitslagcodeomschrijving AS "value.valueCodeableConcept.coding.display",
    CASE
        WHEN NOT (uitslagwaarde ~ '^[-+]?[0-9]*\.?[0-9]+$') AND uitslagcode IS NULL THEN uitslagwaarde
        ELSE NULL
    END AS "value.valueString",
    CASE
        WHEN uitslagwaardeoperator IS NOT NULL THEN
            CASE
                WHEN uitslagwaardeoperator = '<' THEN 'L'
                WHEN uitslagwaardeoperator = '>' THEN 'H'
                ELSE 'N'
            END
    END AS "interpretation.code",
    CASE
        WHEN uitslagwaardeoperator IS NOT NULL THEN 'http://terminology.hl7.org/CodeSystem/v3-ObservationInterpretation'
    END AS "interpretation.system",
    CASE
        WHEN uitslagwaardeoperator IS NOT NULL THEN
            CASE
                WHEN uitslagwaardeoperator = '<' THEN 'Low'
                WHEN uitslagwaardeoperator = '>' THEN 'High'
                ELSE 'Normal'
            END
    END AS "interpretation.display",
    CASE
        WHEN meetmethodecodesysteem IS NOT NULL OR meetmethodecode IS NOT NULL THEN COALESCE(meetmethodecodesysteem, 'unknown')
    END AS "method.coding.system",
    meetmethodecode AS "method.coding.code"
FROM 
    observation_raw
WHERE identificatienummer = :Patient.id
limit 10 ;


-- SELECT
--     identificatienummer as "Patient.id",
--     metingid as "resource_id",
--     metingid AS id,
--     '' AS parent_id,
--     'Observation.category' AS fhir_path
-- FROM 
--     observation_raw
-- WHERE identificatienummer = :Patient.id;

-- SELECT
--     identificatienummer as "Patient.id",
--     metingid as "resource_id",
--     metingid AS id,
--     '' AS parent_id,
--     'Observation.category.coding' AS fhir_path,
--     'http://terminology.hl7.org/CodeSystem/observation-category' AS "system",
--     'laboratory' AS "code",
--     'Laboratory' AS "display"
-- FROM 
--     observation_raw
-- WHERE identificatienummer = :Patient.id;