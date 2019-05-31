import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import Button from 'components/Button';
import Select from 'components/Select';
import Modal from 'components/Modal';
import { createDatabase } from 'actions/spaces';

import './CreateDatabase.scss';

class CreateDatabase extends Component {
  constructor(props) {
    super(props);

    this.state = {
      type: null
    };
  }

  @autobind
  handleConfirmClick() {
    const { space } = this.props;
    this.props.dispatch(createDatabase(space.id, this.state.type));
    this.close();
  }

  @autobind
  handleCancelClick() {
    this.close();
  }

  open() {
    this.dialog && this.dialog.open();
  }

  close() {
    this.dialog && this.dialog.close();
    this.reset();
  }

  reset() {
    this.setState({ type: null });
    this.select.clear();
  }

  render() {
    const { space } = this.props;
    const existingDatabases = space.databases.map(database => database.name);
    const options = [
      { value: 'mongo', label: 'Mongodb' },
      { value: 'redis', label: 'Redis'},
      { value: 'postgres', label: 'Postgres'}
    ].map(option => {
      return {
        ...option,
        isDisabled: existingDatabases.includes(option.value)
      };
    });

    return (
      <Modal
        className="CreateDatabase"
        ref={ref => this.dialog = ref} 
        title="New database"
        width={450}>
        <div className="DialogContent layout vertical">
          <Select
            ref={ref => this.select = ref}
            classNamePrefix="Select" 
            isSearchable={false}
            options={options}
            onChange={value => this.setState({ type: value })}
            value={this.state.type}
            palette="light"
          />
          <div style={{ height: '20px' }} />
          <div className="layout horizontal-reverse center">
            <Button 
              disabled={!this.state.type}
              color="green"
              solid
              onClick={this.handleConfirmClick}>
              Create
            </Button>
            <Button 
              color="grey"
              solid
              secondary
              onClick={this.handleCancelClick}>
              Cancel
            </Button>
          </div>
        </div>
      </Modal>
    );
  }
}

CreateDatabase.propTypes = {
  dispatch: PropTypes.func.isRequired,
  space: PropTypes.object.isRequired
};

export default ReactRedux.connect(() => {
  return {};
}, null, null, { withRef: true })(CreateDatabase);