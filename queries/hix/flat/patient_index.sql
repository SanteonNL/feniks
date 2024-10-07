-- -- Patient data
-- SELECT
--     '123' as id,
--     '' as parent_id,
--     'Patient' as fhir_path,
--     '1990-01-01' as birthDate,
--     'female' as gender;

-- -- Patient names
-- SELECT
--     '123' as id,
--     '' as parent_id,
--     'Patient' as fhir_path,
--     'Albers' as "name[0].family",
--     'Janenalleman' as "name[1].family",
--     JSON_ARRAY('Tommy', 'Smith', 'Doe') as "name[0].given";


SELECT
    '123' as id,
    '' as parent_id,
    'Patient' as fhir_path,
    'MRN' as "identifier[0].type.coding[0].code",
    'Medical Record Number' as "identifier[0].type.coding[0].display",
    'http://terminology.hl7.org/CodeSystem/v2-0203' as "identifier[0].type.coding[0].system",
    'MR' as "identifier[0].type.coding[1].code",
    'Medical Record Number' as "identifier[0].type.coding[1].display",
    'http://terminology.hl7.org/CodeSystem/v2-0203' as "identifier[0].type.coding[1].system",
    '12345' as "identifier[0].value",
    'https://santeon.nl' as "identifier[0].system",
    'official' as "identifier[0].use",
    'SNO' as "identifier[1].type.coding[0].code",
    'Subscriber Number' as "identifier[1].type.coding[0].display",
    'http://terminology.hl7.org/CodeSystem/v2-0203' as "identifier[1].type.coding[0].system",
    '7890' as "identifier[1].value",
    'https://insurance.example.org' as "identifier[1].system",
    'secondary' as "identifier[1].use",
    '1990-01-01' as "identifier[1].period[0].start",
    '1990-01-01' as "birthDate";
