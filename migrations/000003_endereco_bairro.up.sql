-- Neighbourhood, as returned by CEP lookups (ViaCEP); stored for display.
ALTER TABLE enderecos ADD COLUMN bairro text NOT NULL DEFAULT '';
