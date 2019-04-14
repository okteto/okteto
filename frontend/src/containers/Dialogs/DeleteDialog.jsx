import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import Button from 'components/Button';
import Modal from 'components/Modal';
import { deleteEnvironment } from 'actions/environments';
import { deleteDatabase } from 'actions/databases';

import './DeleteDialog.scss';

class DeleteDialog extends Component {
  constructor(props) {
    super(props);

    this.state = {
      environment: null
    };
  }

  @autobind
  handleConfirmClick() {
    if (this.state.type === 'environment') {
      this.props.dispatch(deleteEnvironment(this.state.item));
    } else if (this.state.type === 'database') {
      this.props.dispatch(deleteDatabase(this.state.item));
    }
    this.close();
  }

  @autobind
  handleCancelClick() {
    this.close();
  }

  open(item, type) {
    if (item && ['environment', 'database'].includes(type)) {
      this.setState({ item, type });
      this.dialog && this.dialog.open();
    }
  }

  close() {
    this.dialog && this.dialog.close();
  }

  render() {
    const { item, type } = this.state;
    return (
      <Modal 
        className="DeleteDialog"
        ref={ref => this.dialog = ref} 
        title={`Delete ${type}`}
        width={450}>
        <div className="delete-dialog-content layout vertical">
          <p>
            Are you sure you want to delete {type}&nbsp;
            <strong>{item ? item.name : ''}</strong>?
          </p>
          <div style={{ height: '20px' }} />
          <div className="layout horizontal-reverse center">
            <Button 
              disabled={!item}
              color="red"
              solid
              onClick={this.handleConfirmClick}>
              Delete
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

DeleteDialog.propTypes = {
  dispatch: PropTypes.func.isRequired
};

export default ReactRedux.connect(() => {
  return {};
}, null, null, { withRef: true })(DeleteDialog);